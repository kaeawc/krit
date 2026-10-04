package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.RequestResult
import dev.jasonpearson.krit.fir.FirRuleErrorRecorder
import dev.jasonpearson.krit.fir.isolateRule
import dev.jasonpearson.krit.fir.handleRequestLine
import dev.jasonpearson.krit.fir.SdkLevels
import dev.jasonpearson.krit.fir.parseRequest
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertTrue

class AnalysisSessionModulesTest {
    @TempDir lateinit var tmp: Path

    private fun source(module: String, name: String, code: String): String = tmp.resolve("$module/$name.kt").toFile().apply {
        parentFile.mkdirs()
        writeText(code)
    }.absolutePath

    private fun module(id: String, depends: List<String> = emptyList(), friends: List<String> = emptyList()) =
        ModuleSpec(id, sourceRoots = listOf(tmp.resolve(id).toString()), dependsOn = depends, friends = friends)

    private fun run(modules: List<ModuleSpec>, files: List<String>, block: (BatchResult) -> Unit) {
        val session = AnalysisSession(emptyList(), emptyList())
        try { block(session.analyzeModules(1, modules, files, setOf("PrintlnInProduction"))) }
        finally { session.dispose() }
    }

    @Test fun publicApiAndFriendVisibility() {
        val lib = source("lib", "Lib", "package lib\nfun api() = 1\ninternal fun secret() = 2")
        val app = source("app", "App", "fun app() = lib.api()")
        val test = source("test", "Test", "fun test() = lib.secret()")
        run(listOf(module("app", listOf("lib")), module("test", friends = listOf("lib")), module("lib")), listOf(lib, app, test)) {
            assertEquals(3, it.succeeded, it.toString())
            assertTrue(it.modules.all { m -> m.mode == "module" }, it.toString())
            assertEquals(mapOf(lib to "lib", app to "app", test to "test"), it.decidingModules)
        }
    }

    @Test fun hmppExpectActual() {
        val common = source("kmp/common", "Common", "package multi\nexpect fun foo(): Int\nfun commonUse() = foo()")
        val jvm = source("kmp/jvm", "Jvm", "package multi\nactual fun foo(): Int = 1\nfun use() = foo()")
        val spec = module("kmp").copy(fragments = listOf(
            ModuleFragment("commonMain", listOf(tmp.resolve("kmp/common").toString())),
            ModuleFragment("jvmMain", listOf(tmp.resolve("kmp/jvm").toString()), listOf("commonMain")),
        ))
        run(listOf(spec), listOf(common, jvm)) {
            assertEquals(2, it.succeeded, it.toString())
            assertEquals("module", it.modules.single().mode, it.toString())
        }
    }

    @Test fun gatedHmppDependencyKeepsFragmentOwnershipInFallback() {
        val common = source("kmp/common", "Common", "package multi\nexpect fun foo(): Int")
        val jvm = source("kmp/jvm", "Jvm", "package multi\nactual fun foo(): Int = 1\nfun broken() = unresolved()")
        val app = source("app", "App", "fun app() = multi.foo()")
        val spec = module("kmp").copy(fragments = listOf(
            ModuleFragment("commonMain", listOf(tmp.resolve("kmp/common").toString())),
            ModuleFragment("jvmMain", listOf(tmp.resolve("kmp/jvm").toString()), listOf("commonMain")),
        ))
        run(listOf(spec, module("app", listOf("kmp"))), listOf(common, jvm, app)) {
            assertEquals("merged-fallback", it.modules.first { m -> m.id == "app" }.mode)
            assertEquals(setOf(jvm), it.errorFiles.keys, it.toString())
            assertEquals(2, it.succeeded, it.toString())
        }
    }

    @Test fun perModuleOptInAndManagedArguments() {
        val file = source("opt", "Opt", """
            package opt
            @RequiresOptIn(level = RequiresOptIn.Level.ERROR)
            annotation class ExperimentalApi
            @ExperimentalApi fun api() = 1
            fun use() = api()
        """.trimIndent())
        run(listOf(module("opt")), listOf(file)) { assertTrue(file in it.errorFiles) }
        val args = listOf("-opt-in=opt.ExperimentalApi", "-Werror", "-d", "/invalid/output", "-cp=/invalid", "-jvm-target", "99",
            "-Xplugin=/invalid.jar", "-P", "plugin:bad:option=true")
        assertEquals(listOf("-opt-in=opt.ExperimentalApi"), filterModuleArgs(args))
        run(listOf(module("opt").copy(compilerArgs = args, jvmTarget = "17")), listOf(file)) {
            assertEquals(1, it.succeeded, it.toString())
        }
    }

    @Test fun gatedUpstreamMergesOnlyReachableSources() {
        val lib = source("lib", "Lib", "package lib\nfun api() = 1\nfun broken() = unresolved()\nfun log() { println(1) }")
        val app = source("app", "App", "fun app() = lib.api()")
        val unrelated = source("other", "Other", "package lib\nfun api() = 2")
        run(listOf(module("lib"), module("app", listOf("lib")), module("other")), listOf(lib, app, unrelated)) {
            assertEquals("gated", it.modules.first { m -> m.id == "lib" }.mode)
            assertEquals("merged-fallback", it.modules.first { m -> m.id == "app" }.mode)
            assertTrue(app !in it.errorFiles, it.toString())
            assertEquals(1, it.findings.count { f -> f.path == lib }, it.toString())
            assertEquals(2, it.succeeded, it.toString())
        }
    }

    @Test fun transitiveGatedClosureAndGeneratedSources() {
        val leaf = source("leaf", "Leaf", "package leaf\nfun api() = 1\nfun broken() = missing()")
        val middle = source("middle", "Middle", "package middle\nfun api() = leaf.api()")
        val app = source("app", "App", "fun app() = middle.api() + generated.value()")
        source("generated", "Generated", "package generated\nfun value() = 2")
        run(listOf(module("app", listOf("middle")).copy(generatedSourceRoots = listOf(tmp.resolve("generated").toString())),
            module("middle", listOf("leaf")), module("leaf")), listOf(leaf, middle, app)) {
            assertEquals(2, it.succeeded, it.toString())
            assertEquals(setOf("middle", "app"), it.modules.filter { m -> m.mode == "merged-fallback" }.map { m -> m.id }.toSet())
            assertEquals(setOf(leaf), it.errorFiles.keys)
        }
    }

    @Test fun cyclesDoNotSwallowIndependentOrDownstreamModules() {
        val a = source("a", "A", "package a\nfun a(): Int = b.b()")
        val b = source("b", "B", "package b\nfun b() = 1")
        val c = source("c", "C", "fun c() = a.a()")
        val d = source("d", "D", "fun d() = 2")
        run(listOf(module("c", listOf("a")), module("a", listOf("b")), module("b", listOf("a")), module("d")), listOf(a,b,c,d)) {
            assertEquals(4, it.succeeded, it.toString())
            assertEquals(setOf("a", "b"), it.modules.filter { m -> m.mode == "skipped" }.map { m -> m.id }.toSet())
            assertTrue(it.modules.filter { m -> m.mode == "skipped" }.all { m -> m.firstError!!.contains("cycle") })
            assertEquals("module", it.modules.first { m -> m.id == "c" }.mode)
            assertEquals("module", it.modules.first { m -> m.id == "d" }.mode)
        }
    }

    @Test fun daemonReusesLibButInvalidatesAllDependentsOnAnyEdit() {
        val lib = source("lib", "Lib", "package lib\nfun api() = 1")
        val app = source("app", "App", "fun app() = lib.api()")
        val session = AnalysisSession(emptyList(), emptyList())
        val json = """{"id":1,"command":"analyzeModules","modules":[
            {"id":"lib","sourceRoots":["${tmp.resolve("lib")}"]},
            {"id":"app","sourceRoots":["${tmp.resolve("app")}"],"dependsOn":["lib"]}
            ],"checkFiles":["$lib","$app"],"rules":["PrintlnInProduction"]}"""
        try {
            fun request() {
                val response = handleRequestLine(json, session, 0) as RequestResult.Response
                assertTrue(response.json.contains("\"succeeded\":2"), response.json)
            }
            request()
            assertEquals(mapOf("lib" to 1, "app" to 1), session.moduleCompilationCounts)
            source("app", "App", "fun app() = lib.api() + 1")
            request()
            assertEquals(mapOf("lib" to 1, "app" to 2), session.moduleCompilationCounts)
            // Body-only changes still invalidate the full reverse closure.
            source("lib", "Lib", "package lib\nfun api() = 2")
            request()
            assertEquals(mapOf("lib" to 2, "app" to 3), session.moduleCompilationCounts)
            request()
            assertEquals(mapOf("lib" to 2, "app" to 3), session.moduleCompilationCounts)
        } finally { session.dispose() }
    }

    @Test fun cachedDependencyAndFriendOutputsSurviveDownstreamOnlyRecompile() {
        for (friendEdge in listOf(false, true)) {
            val lib = source("lib", "Lib", "package lib\nclass Api")
            val mid = source("mid", "Mid", "package mid\nclass Middle : java.io.Serializable { fun api() = lib.Api() }")
            val app = source("app", "App", "fun app() = mid.Middle()")
            val modules = listOf(module("lib"), module("mid", listOf("lib")),
                if (friendEdge) module("app", friends = listOf("mid")) else module("app", listOf("mid")))
            val session = AnalysisSession(emptyList(), emptyList())
            try {
                fun request() {
                    val result = session.analyzeModules(1, modules, listOf(lib, mid, app), emptySet())
                    assertEquals(3, result.succeeded, result.toString())
                    assertTrue(result.modules.all { it.mode == "module" }, result.toString())
                }
                request()
                val outputs = session.moduleOutputDirectories.filterKeys { it != "app" }
                val classes = outputs.mapValues { (_, dir) ->
                    assertTrue(dir.isDirectory, dir.toString())
                    dir.walkTopDown().filter { it.isFile && it.extension == "class" }
                        .associate { it.relativeTo(dir).path to it.readBytes().toList() }
                        .also { assertTrue(it.isNotEmpty(), "No classes in $dir") }
                }
                source("app", "App", "fun app() = mid.Middle().toString()")
                request()
                assertEquals(mapOf("lib" to 1, "mid" to 1, "app" to 2), session.moduleCompilationCounts)
                for ((id, dir) in outputs) {
                    assertEquals(dir, session.moduleOutputDirectories.getValue(id))
                    assertTrue(dir.isDirectory, "Cached output disappeared: $dir")
                    for ((path, bytes) in classes.getValue(id)) {
                        val file = dir.resolve(path)
                        assertTrue(file.isFile, "Cached class disappeared: $file")
                        assertEquals(bytes, file.readBytes().toList(), "Cached class changed: $file")
                    }
                }
            } finally { session.dispose() }
        }
    }

    @Test fun checkerExecutionIsScopedToRequestedModuleFiles() {
        val own = source("app", "App", "fun app() = 1")
        val upstream = source("lib", "Lib", "fun lib() = 1")
        val recorder = FirRuleErrorRecorder(setOf(own))
        var calls = 0
        isolateRule(recorder, "Rule", { upstream }) { calls++ }
        isolateRule(recorder, "Rule", { own }) { calls++ }
        assertEquals(1, calls)
        isolateRule(FirRuleErrorRecorder(emptySet()), "Rule", { own }) { calls++ }
        assertEquals(1, calls)
        isolateRule(FirRuleErrorRecorder(), "Rule", { upstream }) { calls++ }
        assertEquals(2, calls)
    }

    @Test fun moduleProtocolDoesNotLeakNestedFields() {
        val request = parseRequest("""{"id":3,"command":"analyzeModules","modules":[{"id":":lib:main","classpath":["nested.jar"],"sourceRoots":["/src"]}],"checkFiles":["/src/A.kt"],"ruleConfigs":{"Rule":{"classpath":"option"}}}""")
        assertEquals(emptyList(), request.classpath)
        assertEquals(listOf("nested.jar"), request.modules.single().classpath)
        assertEquals("/src/A.kt", request.files.single().path)
        assertEquals("option", request.ruleConfigs["Rule"]?.get("classpath"))
    }
    @Test fun transitivePublicSuperclassAndFriendClasspath() {
        val lib = source("lib", "Lib", "package lib; open class Base { fun value() = 1 }; internal fun secret() = 1")
        val mid = source("mid", "Mid", "package mid; class Middle : lib.Base()")
        val app = source("app", "App", "fun app() = mid.Middle().value()")
        for (friend in listOf(false, true)) {
            run(listOf(module("lib"), module("mid", listOf("lib")),
                if (friend) module("app", friends = listOf("mid")) else module("app", listOf("mid"))), listOf(lib, mid, app)) {
                assertEquals(3, it.succeeded, it.toString())
                assertTrue(it.modules.all { m -> m.mode == "module" }, it.toString())
            }
        }
    }

    @Test fun findingsChangesStayLocalAndDoNotPropagate() {
        val lib = source("lib", "Lib", "package lib; fun api() = 1")
        val app = source("app", "App", "fun app() { println(lib.api()) }")
        val session = AnalysisSession(emptyList(), emptyList())
        val modules = listOf(module("lib"), module("app", listOf("lib")))
        try {
            fun check(files: List<String>, tests: Set<String> = emptySet()) =
                session.analyzeModules(1, modules, files, setOf("PrintlnInProduction"), testFiles = tests)
            check(listOf(lib, app))
            check(listOf(lib, app), setOf(app))
            assertEquals(mapOf("lib" to 1, "app" to 2), session.moduleCompilationCounts)
            check(listOf(lib), setOf(app))
            assertEquals(mapOf("lib" to 1, "app" to 3), session.moduleCompilationCounts)
            // An upstream findings-only change cannot dirty an unchanged downstream verdict.
            check(listOf(lib), setOf(lib))
            assertEquals(mapOf("lib" to 2, "app" to 3), session.moduleCompilationCounts)
        } finally { session.dispose() }
    }

    @Test fun sdkLevelChangeRechecksOnlyTheOwningModule() {
        val lib = source("lib", "Lib", "package lib; fun api() = 1")
        val app = source("app", "App", "fun app() { println(lib.api()) }")
        val session = AnalysisSession(emptyList(), emptyList())
        val modules = listOf(module("lib"), module("app", listOf("lib")))
        try {
            fun check(levels: Map<String, SdkLevels>) =
                session.analyzeModules(1, modules, listOf(lib, app), setOf("PrintlnInProduction"), sdkLevels = levels)
            check(mapOf(app to SdkLevels(21, 33)))
            check(mapOf(app to SdkLevels(21, 33)))
            assertEquals(mapOf("lib" to 1, "app" to 1), session.moduleCompilationCounts)
            // A checker can read the levels, so a targetSdk change must not replay the old verdict.
            check(mapOf(app to SdkLevels(21, 34)))
            assertEquals(mapOf("lib" to 1, "app" to 2), session.moduleCompilationCounts)
        } finally { session.dispose() }
    }

    @Test fun sharedClasspathBytesAreMemoizedAcrossRequests() {
        val jar = tmp.resolve("shared.jar").toFile()
        java.util.jar.JarOutputStream(jar.outputStream()).use { }
        val a = source("a", "A", "fun a() = 1")
        val b = source("b", "B", "fun b() = 2")
        val session = AnalysisSession(emptyList(), emptyList())
        val modules = listOf(module("a"), module("b")).map { it.copy(classpath = listOf(jar.path)) }
        try {
            session.analyzeModules(1, modules, listOf(a, b), emptySet())
            assertEquals(1, session.moduleRunner.classpathHashReads)
            session.analyzeModules(2, modules, listOf(a, b), emptySet())
            assertEquals(1, session.moduleRunner.classpathHashReads)
            jar.setLastModified(jar.lastModified() + 2000)
            session.analyzeModules(3, modules, listOf(a, b), emptySet())
            assertEquals(2, session.moduleRunner.classpathHashReads)
        } finally { session.dispose() }
    }

    @Test fun cleanFallbackOwnershipAndBoundedLegacyRecovery() {
        val files = (0..4).map { n -> source("m$n", "M$n",
            "package m$n; fun api(): Int = " + if (n == 0) "1; fun broken() = missing()" else "m${n - 1}.api()") }
        val modules = (0..4).map { n -> module("m$n", if (n == 0) emptyList() else listOf("m${n - 1}")) }
        val session = AnalysisSession(emptyList(), emptyList())
        try {
            val result = session.analyzeModules(1, modules, files, emptySet())
            assertEquals(setOf(files.first()), result.errorFiles.keys, result.toString())
            assertTrue(result.modules.drop(1).all { it.mode == "merged-fallback" && it.firstError == null }, result.toString())
            assertEquals((session.moduleRunner.compiledSources.getValue("m2") + files[3]).map { java.io.File(it).canonicalPath }.toSet(),
                session.moduleRunner.compiledSources.getValue("m3").map { java.io.File(it).canonicalPath }.toSet())
            session.moduleRunner.mergeFileLimit = 1
            source("m4", "M4", "package m4; fun api(): Int = m3.api() + 1")
            val capped = session.analyzeModules(2, modules, files, emptySet())
            assertEquals("legacy-fallback", capped.modules.last().mode)
            assertTrue(capped.modules.last().firstError!!.contains("limit"))
            assertTrue(files.last() !in capped.errorFiles, capped.toString())
        } finally { session.dispose() }
    }

    @Test fun failedCompileEvictsCacheAndRetriesSameInputs() {
        val file = source("lib", "Lib", "fun api() = 1")
        val session = AnalysisSession(emptyList(), emptyList())
        try {
            val modules = listOf(module("lib"))
            session.analyzeModules(1, modules, listOf(file), emptySet())
            val old = session.moduleOutputDirectories.getValue("lib")
            source("lib", "Lib", "fun api() = 2")
            session.moduleRunner.beforeCompile = { invocation ->
                invocation.output.resolve("partial.class").writeText("partial")
                error("injected compiler failure")
            }
            kotlin.test.assertFailsWith<IllegalStateException> {
                session.analyzeModules(2, modules, listOf(file), emptySet())
            }
            assertTrue(session.moduleOutputDirectories.isEmpty())
            assertTrue(!old.exists())
            source("lib", "Lib", "fun api() = 1")
            session.moduleRunner.beforeCompile = {}
            assertEquals(1, session.analyzeModules(3, modules, listOf(file), emptySet()).succeeded)
            assertEquals(3, session.moduleCompilationCounts["lib"])
            assertTrue(session.moduleOutputDirectories.getValue("lib").walkTopDown().any { it.extension == "class" })
        } finally { session.dispose() }
    }

    @Test fun multiLeafHmppAssignsGeneratedSourceWithSubset() {
        source("kmp/common", "Common", "package multi; fun common() = 1")
        source("kmp/intermediate", "Shared", "package multi; fun shared() = common()")
        source("kmp/jvm", "Jvm", "package multi; fun jvm() = shared()")
        source("kmp/android", "Android", "package multi; fun android() = shared()")
        val generated = source("generated", "Generated", "package multi; fun generated() = jvm()")
        val fragments = listOf(
            ModuleFragment("commonMain", listOf(tmp.resolve("kmp/common").toString())),
            ModuleFragment("nonAndroidMain", listOf(tmp.resolve("kmp/intermediate").toString()), listOf("commonMain")),
            ModuleFragment("jvmMain", listOf(tmp.resolve("kmp/jvm").toString()), listOf("nonAndroidMain")),
            ModuleFragment("androidMain", listOf(tmp.resolve("kmp/android").toString()), listOf("nonAndroidMain")),
        )
        val spec = module("kmp").copy(fragments = fragments, generatedSourceRoots = listOf(tmp.resolve("generated").toString()))
        val args = org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments()
        ModuleCompilation(moduleSources(spec), tmp.resolve("out").toFile(), emptyList(), emptyList(), spec, emptyList()).configure(args)
        assertTrue(args.fragmentSources!!.contains("jvmMain:${java.io.File(generated).canonicalPath}"))
        run(listOf(spec), listOf(generated)) {
            assertEquals(1, it.succeeded, it.toString())
            assertEquals("module", it.modules.single().mode, it.toString())
        }
    }

    @Test fun removedAndRegroupedOutputsAreDeleted() {
        val a = source("a", "A", "fun a() = 1")
        val b = source("b", "B", "fun b() = 2")
        val session = AnalysisSession(emptyList(), emptyList())
        try {
            session.analyzeModules(1, listOf(module("a"), module("b")), listOf(a, b), emptySet())
            val old = session.moduleOutputDirectories.values.toList()
            session.analyzeModules(2, listOf(module("a", listOf("b")), module("b", listOf("a"))), listOf(a, b), emptySet())
            assertTrue(old.none { it.exists() })
            val cycle = session.moduleOutputDirectories.getValue("b")
            session.analyzeModules(3, listOf(module("a")), listOf(a), emptySet())
            assertTrue(!cycle.exists())
            assertEquals(setOf("a"), session.moduleOutputDirectories.keys)
        } finally { session.dispose() }
    }

    @Test fun legacyRebuildPreservesModuleCache() {
        val file = source("lib", "Lib", "fun api() = 1")
        val legacy = source("legacy", "Legacy", "fun legacy() = 1")
        var session = AnalysisSession(emptyList(), emptyList())
        try {
            val modules = listOf(module("lib"))
            session.analyzeModules(1, modules, listOf(file), emptySet())
            val result = handleRequestLine("""{"id":2,"command":"check","sourceDirs":["${tmp.resolve("legacy")}"],"files":["$legacy"]}""", session, 0)
                as RequestResult.SessionRebuilt
            session = result.newSession
            session.analyzeModules(3, modules, listOf(file), emptySet())
            assertEquals(1, session.moduleCompilationCounts["lib"])
        } finally { session.dispose() }
    }

    @Test fun nestedSourceOwnershipIgnoresDeclarationOrder() {
        val outer = source("src", "Outer", "fun outer() = 1")
        val nested = source("src/test/kotlin", "Nested", "fun nested() { println(1) }")
        val modules = listOf(module("src"), module("src/test/kotlin"))
        for (order in listOf(modules, modules.reversed())) run(order, listOf(outer, nested)) {
            assertEquals("src/test/kotlin", it.decidingModules[nested])
            assertEquals(1, it.findings.count { f -> f.path == nested }, it.toString())
            assertEquals(2, it.succeeded, it.toString())
        }
    }

    @Test fun abandonedOutputSweepPreservesLiveAndRecentOwners() {
        val parent = tmp.resolve("sessions").toFile().apply { mkdirs() }
        val dead = parent.resolve("krit-fir-modules-9223372036854775806-old").apply { mkdir() }
        val live = parent.resolve("krit-fir-modules-${ProcessHandle.current().pid()}-live").apply { mkdir() }
        val recent = parent.resolve("krit-fir-modules-9223372036854775806-new").apply { mkdir() }
        val now = System.currentTimeMillis()
        dead.setLastModified(now - 48 * 60 * 60 * 1000L)
        live.setLastModified(now - 48 * 60 * 60 * 1000L)
        sweepAbandonedModuleDirectories(parent, now)
        assertTrue(!dead.exists())
        assertTrue(live.exists())
        assertTrue(recent.exists())
    }

}
