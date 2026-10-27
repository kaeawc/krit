package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.cliCompilerPlugins
import dev.jasonpearson.krit.fir.parseModuleRequest
import dev.jasonpearson.krit.fir.parseRequest
import dev.jasonpearson.krit.fir.sessionNeedsRebuild
import dev.jasonpearson.krit.fir.withWarnings
import org.junit.jupiter.api.AfterEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import java.util.zip.ZipEntry
import java.util.zip.ZipOutputStream
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotEquals
import kotlin.test.assertNotNull
import kotlin.test.assertTrue

/**
 * The project's Kotlin compiler plugins (#947): a module that relies on plugin-generated declarations
 * (`Foo.serializer()` from kotlinx.serialization) compiles cleanly only when the plugin is loaded, and the plugin set
 * is part of every compile identity so a plugin change invalidates the incremental state.
 */
class CompilerPluginsTest {
    @TempDir lateinit var tmp: Path

    private val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
    private val serializationPlugin = File(requireNotNull(System.getProperty("krit.fir.test.serializationPlugin"))).absolutePath
    private val serializationRuntime = File(requireNotNull(System.getProperty("krit.fir.test.serializationRuntime"))).absolutePath
    private val sessions = ArrayList<AnalysisSession>()
    private var counter = 0

    @AfterEach fun cleanUp() = sessions.forEach { it.dispose() }

    private fun sourceDir(vararg files: Pair<String, String>): File =
        tmp.resolve("src-${counter++}").toFile().apply {
            mkdirs()
            for ((name, text) in files) File(this, name).writeText(text)
        }

    private fun session(
        dir: File, plugins: CompilerPlugins = CompilerPlugins.EMPTY, planning: Boolean = false,
    ): AnalysisSession = AnalysisSession(
        listOf(dir.path), listOf(stdlib, serializationRuntime), "", emptyList(),
        incrementalRecording = planning, incrementalPlanning = planning, compilerPlugins = plugins,
    ).also { it.snapshotRootOverride = tmp.resolve("snapshots").toFile(); sessions += it }

    private fun serialization(options: List<String> = emptyList()) = CompilerPlugins.of(listOf(serializationPlugin), options)

    private fun AnalysisSession.checkAll(dir: File) =
        check(1, dir.listFiles()!!.filter { it.extension == "kt" }.sorted().map { FileRef(it.path) }, setOf("ProtocolProbe"))

    @Test fun serializableModuleCompilesCleanlyOnlyWithTheSerializationPlugin() {
        val dir = sourceDir("Model.kt" to SERIALIZABLE, "Use.kt" to USES_SERIALIZER)
        val without = session(dir).checkAll(dir)
        val errors = without.errorFiles.mapKeys { File(it.key).name }
        assertTrue("Use.kt" in errors, "without the plugin serializer() is unresolved: $errors")
        assertTrue("serializer" in errors.getValue("Use.kt"), errors.toString())

        val with = session(dir, serialization())
        val result = with.checkAll(dir)
        assertEquals(emptyMap(), result.errorFiles, "the plugin generates Foo.serializer()")
        assertEquals(emptyList(), with.compilerPluginWarnings)
        assertTrue(with.effectivePlugins.classpath.contains(serializationPlugin))
    }

    @Test fun theOracleAndTheSharedCheckCompileWithThePlugins() {
        val dir = sourceDir("Model.kt" to SERIALIZABLE, "Use.kt" to USES_SERIALIZER)
        val s = session(dir, serialization())
        val files = dir.listFiles()!!.filter { it.extension == "kt" }.map { it.path }
        val request = parseRequest("""{"id":3,"command":"check","files":${files.joinToString(",", "[", "]") { "\"$it\"" }},"rules":["ProtocolProbe"]}""")
        val (_, checked) = s.analyzeWithCheck(files, request)
        assertNotNull(checked, "the check shares the oracle's compile")
        assertEquals(emptyMap(), checked.errorFiles)
    }

    @Test fun moduleCompilationsUseTheirModulesPlugins() {
        val dir = sourceDir("Model.kt" to SERIALIZABLE, "Use.kt" to USES_SERIALIZER)
        val files = dir.listFiles()!!.filter { it.extension == "kt" }.map { it.path }
        val module = ModuleSpec("m", sourceRoots = listOf(dir.path), classpath = listOf(stdlib, serializationRuntime))
        val host = AnalysisSession(emptyList(), emptyList()).also { sessions += it }
        val without = host.analyzeModules(1, listOf(module), files, setOf("ProtocolProbe"))
        assertTrue(without.errorFiles.isNotEmpty(), without.errorFiles.toString())
        val with = host.analyzeModules(2, listOf(module.copy(compilerPluginClasspath = listOf(serializationPlugin))), files,
            setOf("ProtocolProbe"))
        assertEquals(emptyMap(), with.errorFiles, "a plugin change recompiles the module with the plugin")
    }

    @Test fun compileKeyFollowsPluginJarsAndOptions() {
        val dir = sourceDir("Model.kt" to SERIALIZABLE)
        val copy = File(tmp.toFile(), "copy/serialization.jar").apply { parentFile.mkdirs() }
        File(serializationPlugin).copyTo(copy)
        val none = session(dir).compileKeyNow()
        val plugin = session(dir, CompilerPlugins.of(listOf(copy.path), emptyList())).compileKeyNow()
        assertNotEquals(none, plugin, "adding a plugin changes the key")
        assertEquals(plugin, session(dir, CompilerPlugins.of(listOf(copy.path), emptyList())).compileKeyNow(), "stable")
        val option = session(dir, CompilerPlugins.of(listOf(copy.path),
            listOf("plugin:org.jetbrains.kotlinx.serialization:disableIntrinsic=true"))).compileKeyNow()
        assertNotEquals(plugin, option, "a plugin option changes the key")
        // The same path with other contents (a plugin upgrade) changes the key too.
        copy.appendBytes(ByteArray(16))
        copy.setLastModified(copy.lastModified() + 5_000)
        val rebuilt = session(dir, CompilerPlugins.of(listOf(copy.path), emptyList())).compileKeyNow()
        assertNotEquals(plugin, rebuilt, "a rebuilt plugin jar changes the key")
    }

    @Test fun aPluginTheCompilerCannotLoadIsLeftOutWithAWarning() {
        val broken = File(tmp.toFile(), "broken-plugin.jar")
        ZipOutputStream(broken.outputStream()).use { zip ->
            zip.putNextEntry(ZipEntry("META-INF/services/org.jetbrains.kotlin.compiler.plugin.CompilerPluginRegistrar"))
            zip.write("com.example.DoesNotExistRegistrar\n".toByteArray())
            zip.closeEntry()
        }
        val dir = sourceDir("A.kt" to "package p\n\nfun protocolProbe() {}\nfun a() = protocolProbe()\n")
        val s = session(dir, CompilerPlugins.of(listOf(broken.path), emptyList()))
        val result = s.checkAll(dir)
        assertEquals(emptyMap(), result.errorFiles, "the session analyzes as without plugins")
        assertTrue(s.effectivePlugins.isEmpty)
        val warning = s.compilerPluginWarnings.single()
        assertTrue("compiler plugins not loaded" in warning && "broken-plugin.jar" in warning, warning)
        assertTrue("compiler plugins not loaded" in withWarnings("{\"id\":1}", s, ""), "the warning reaches the response")
        assertEquals(session(dir).compileKeyNow(), s.compileKeyNow(), "keyed as the compile it runs: without plugins")
    }

    @Test fun pluginSetsAreValidated() {
        val missing = File(tmp.toFile(), "missing.jar").path
        val plugins = CompilerPlugins.of(
            listOf(serializationPlugin, missing, serializationPlugin, " "),
            listOf("plugin:id:key=value", "-Xplugin=x.jar", "plugin:id:key=value", "plugin:id"),
        )
        assertEquals(listOf(serializationPlugin), plugins.classpath)
        assertEquals(listOf("plugin:id:key=value"), plugins.options)
        assertEquals(3, plugins.warnings.size, plugins.warnings.toString())
        assertTrue(plugins.warnings.any { "missing.jar" in it && "not found" in it })
        assertTrue(plugins.warnings.any { "-Xplugin=x.jar" in it })

        val orphan = CompilerPlugins.of(emptyList(), listOf("plugin:id:key=value"))
        assertTrue(orphan.isEmpty)
        assertEquals(emptyList(), orphan.options)
        assertTrue(orphan.warnings.single().contains("no compiler plugin jar"))
        assertEquals("", CompilerPlugins.EMPTY.identity)
    }

    @Test fun requestsAndCommandLinesCarryThePlugins() {
        val request = parseRequest("""{"id":1,"command":"check","compilerPluginClasspath":["$serializationPlugin"],"compilerPluginOptions":["plugin:a:b=c"]}""")
        assertEquals(listOf(serializationPlugin), request.compilerPluginClasspath)
        assertEquals(listOf("plugin:a:b=c"), request.compilerPluginOptions)

        val dir = sourceDir("A.kt" to "package p\n")
        val plain = session(dir)
        assertTrue(sessionNeedsRebuild(request, plain), "a different plugin set rebuilds the session")
        assertFalse(sessionNeedsRebuild(parseRequest("""{"id":1,"command":"check"}"""), plain), "absent keeps the session's")
        val same = session(dir, CompilerPlugins.of(listOf(serializationPlugin), listOf("plugin:a:b=c")))
        assertFalse(sessionNeedsRebuild(request, same))

        val cli = cliCompilerPlugins(arrayOf("--compiler-plugin", serializationPlugin, "--compiler-plugin-option", "plugin:a:b=c"))
        assertEquals(same.compilerPlugins.identity, cli.identity)

        val modules = parseModuleRequest("""{"id":1,"command":"analyzeModules","compilerPluginClasspath":["$serializationPlugin"],
            "modules":[{"id":"m","compilerPluginClasspath":["$serializationPlugin"],"compilerPluginOptions":["plugin:a:b=c"]}]}""")!!
        assertEquals(listOf(serializationPlugin), modules.compilerPluginClasspath)
        assertEquals(listOf(serializationPlugin), modules.modules.single().compilerPluginClasspath)
        assertEquals(listOf("plugin:a:b=c"), modules.modules.single().compilerPluginOptions)
    }

    @Test fun thePlannerSaysWhyAModuleWithCompileErrorsHasNoSnapshot() {
        val dir = sourceDir("Model.kt" to SERIALIZABLE, "Use.kt" to USES_SERIALIZER)
        val s = session(dir, planning = true)
        assertEquals("cold", s.checkAll(dir).plan?.reason)
        val warm = assertNotNull(s.checkAll(dir).plan)
        assertEquals("full", warm.path)
        assertEquals("slice_unavailable", warm.reason)
        assertEquals("COMPILE_ERRORS: compile errors in 1 file", warm.detail)

        // With the plugin the module is clean: it is snapshotted and the next check is served incrementally.
        val clean = session(dir, serialization(), planning = true)
        assertEquals("cold", clean.checkAll(dir).plan?.reason)
        File(dir, "Use.kt").writeText(USES_SERIALIZER + "\nfun more() = 1\n")
        assertEquals("incremental", clean.checkAll(dir).plan?.path)
    }

    private companion object {
        const val SERIALIZABLE = "package p\n\nimport kotlinx.serialization.Serializable\n\n@Serializable\ndata class Foo(val x: Int)\n"
        const val USES_SERIALIZER = "package p\n\nimport kotlinx.serialization.KSerializer\n\nfun fooSerializer(): KSerializer<Foo> = Foo.serializer()\n"
    }
}
