package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.parseModuleRequest
import dev.jasonpearson.krit.fir.parseRequest
import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.config.LanguageVersion
import org.junit.jupiter.api.Test
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotEquals
import kotlin.test.assertNull
import kotlin.test.assertTrue

class KotlinCompilerSettingsTest {
    @Test fun parsesVersionsOptInsAndAllowlistedFlags() {
        val s = KotlinCompilerSettings.parse(listOf(
            "-language-version=2.1", "-api-version", "2.0", "-opt-in=b.B", "-Xopt-in=a.A", "-opt-in", "b.B", "-progressive",
            "-Xcontext-parameters", "-Xannotation-default-target=param-property", "-XXLanguage:+WhenGuards",
            "-XXLanguage:-WhenGuards", "-XXLanguage:+ContextParameters", "-Xjvm-default=all",
        ))
        assertEquals("2.1", s.languageVersion)
        assertEquals("2.0", s.apiVersion)
        assertEquals(listOf("a.A", "b.B"), s.optIns)
        assertTrue(s.progressive)
        // The last occurrence of a flag or feature toggle wins and keeps its position.
        assertEquals(listOf("-Xcontext-parameters", "-Xannotation-default-target=param-property",
            "-XXLanguage:-WhenGuards", "-XXLanguage:+ContextParameters", "-Xjvm-default=all"), s.flags)
        assertTrue(s.warnings.isEmpty(), s.warnings.toString())
        // The rendered form is what Go and the helpers exchange; it parses back to the same settings.
        assertEquals(s.copy(warnings = emptyList()), KotlinCompilerSettings.parse(s.arguments).copy(warnings = emptyList()))
    }

    @Test fun protectedArgumentsAreNeverApplied() {
        val args = listOf("-d", "/out", "-classpath=/a.jar", "-cp", "/b.jar", "-jvm-target", "1.8", "-module-name=m",
            "-Xplugin=/p.jar", "-P", "plugin:x:y=z", "-Xfriend-paths=/f", "-Xfragments=a,b", "-Xfragment-sources=a:/a.kt",
            "-Xfragment-refines=a:b", "-Xmulti-platform", "-Xjdk-release=8", "-no-stdlib", "-Xcommon-sources=/c.kt")
        val s = KotlinCompilerSettings.parse(args)
        assertTrue(s.isEmpty, s.toString())
        assertEquals(
            listOf("-d /out", "-classpath=/a.jar", "-cp /b.jar", "-jvm-target 1.8", "-module-name=m", "-Xplugin=/p.jar",
                "-P plugin:x:y=z", "-Xfriend-paths=/f", "-Xfragments=a,b", "-Xfragment-sources=a:/a.kt", "-Xfragment-refines=a:b",
                "-Xmulti-platform", "-Xjdk-release=8", "-no-stdlib", "-Xcommon-sources=/c.kt"),
            s.warnings.map { it.removePrefix("ignoring compiler argument ").removeSuffix(" (managed by krit)") },
        )
        // Even applied to arguments that already carry krit's own values, nothing changes.
        val target = K2JVMCompilerArguments().apply {
            destination = "/krit/out"; classpath = "/krit/cp"; moduleName = "krit_m"; friendPaths = arrayOf("/krit/friend")
        }
        s.applyTo(target)
        assertEquals("/krit/out", target.destination)
        assertEquals("/krit/cp", target.classpath)
        assertEquals("krit_m", target.moduleName)
        assertEquals(listOf("/krit/friend"), target.friendPaths?.toList())
    }

    @Test fun malformedAndUnsupportedArgumentsAreReported() {
        val s = KotlinCompilerSettings.parse(listOf(
            "-language-version=latest", "-api-version=", "-opt-in=not a marker", "-Xcontext-parameters=true",
            "-Xannotation-default-target", "-Xannotation-default-target=a b", "-XXLanguage:+Bad Feature", "-Werror",
            "-Xno-param-assertions", "-language-version",
        ))
        assertTrue(s.isEmpty, s.toString())
        assertEquals(10, s.warnings.size, s.warnings.toString())
        assertTrue(s.warnings.any { "unsupported version value" in it })
        assertTrue(s.warnings.any { "flag takes no value" in it })
        assertTrue(s.warnings.any { "flag needs a simple value" in it })
        assertTrue(s.warnings.any { "-Werror" in it && "not an analysis-semantic" in it })
    }

    @Test fun versionsTheEmbeddedCompilerCannotHonorAreDropped() {
        val s = KotlinCompilerSettings.parse(listOf("-language-version=1.9", "-api-version=1.9", "-opt-in=a.A"))
        assertNull(s.languageVersion)
        assertNull(s.apiVersion)
        assertEquals(listOf("a.A"), s.optIns)
        assertEquals(2, s.warnings.size, s.warnings.toString())
        assertNull(KotlinCompilerSettings.parse(listOf("-language-version=9.9")).languageVersion)
        assertEquals("2.1", KotlinCompilerSettings.parse(listOf("-language-version=2.1")).languageVersion)
    }

    @Test fun identityIsCanonicalAndChangesWithEverySemanticSetting() {
        val base = KotlinCompilerSettings.parse(listOf("-language-version=2.1", "-api-version=2.1", "-opt-in=a.A", "-Xcontext-parameters"))
        // Argument order, duplicate opt-ins and rejected extras are not semantic.
        assertEquals(base.identity, KotlinCompilerSettings.parse(listOf("-Xcontext-parameters", "-opt-in=a.A", "-api-version=2.1",
            "-language-version=2.1", "-opt-in=a.A", "-Werror")).identity)
        val variants = listOf(
            listOf("-language-version=2.2", "-api-version=2.1", "-opt-in=a.A", "-Xcontext-parameters"),
            listOf("-language-version=2.1", "-api-version=2.0", "-opt-in=a.A", "-Xcontext-parameters"),
            listOf("-language-version=2.1", "-api-version=2.1", "-opt-in=a.A", "-opt-in=b.B", "-Xcontext-parameters"),
            listOf("-language-version=2.1", "-api-version=2.1", "-opt-in=a.A"),
            listOf("-language-version=2.1", "-api-version=2.1", "-opt-in=a.A", "-Xcontext-parameters", "-progressive"),
        )
        for (v in variants) assertNotEquals(base.identity, KotlinCompilerSettings.parse(v).identity, v.toString())
        assertEquals("", KotlinCompilerSettings().identity)
    }

    @Test fun appliesToCompilerArgumentsWithoutTouchingOwnedFields() {
        val args = K2JVMCompilerArguments().apply { optIn = arrayOf("existing.Marker") }
        KotlinCompilerSettings.parse(listOf(
            "-language-version=2.1", "-api-version=2.0", "-opt-in=a.A", "-progressive", "-Xwhen-guards",
            "-XXLanguage:+ContextParameters", "-Xannotation-default-target=param-property",
        )).applyTo(args)
        assertEquals("2.1", args.languageVersion)
        assertEquals("2.0", args.apiVersion)
        assertEquals(listOf("existing.Marker", "a.A"), args.optIn?.toList())
        assertTrue(args.progressiveMode)
        assertTrue(args.whenGuards)
        assertEquals("param-property", args.annotationDefaultTarget)
        assertTrue(args.internalArguments.isNotEmpty(), "-XXLanguage feature toggle should be recorded")
    }

    // The Go allowlist (internal/gradlemodel/kotlin_settings.go) and this one must stay equal;
    // the Go test counts the `allowlist:` tags and looks for each quoted flag name here.
    @Test fun allowlistMatchesTheTaggedSourceLines() {
        val source = File("src/main/kotlin/dev/jasonpearson/krit/fir/runner/KotlinCompilerSettings.kt").readText()
        val tagged = source.lines().filter { "// allowlist:" in it }
        assertEquals(KotlinCompilerSettings.ALLOWLIST.size, tagged.size)
        for (name in KotlinCompilerSettings.ALLOWLIST.keys) assertTrue(tagged.any { "\"$name\"" in it }, name)
        assertTrue(KotlinCompilerSettings.ALLOWLIST.keys.none { KotlinCompilerSettings.isProtected(it) })
    }

    @Test fun nullabilityAnnotationValuesMayContainAtSigns() {
        val s = KotlinCompilerSettings.parse(listOf(
            "-Xnullability-annotations=@org.jspecify.annotations:strict", "-Xjsr305=@org.example.Nullable:warn", "-Xjsr305=strict",
        ))
        assertTrue(s.warnings.isEmpty(), s.warnings.toString())
        // The last -Xjsr305 wins, and '@' stays valid only inside a value.
        assertEquals(listOf("-Xnullability-annotations=@org.jspecify.annotations:strict", "-Xjsr305=strict"), s.flags)
        assertTrue(KotlinCompilerSettings.parse(listOf("@-Xjsr305=strict")).isEmpty)
    }

    @Test fun progressiveFalseTurnsProgressiveModeOff() {
        assertTrue(KotlinCompilerSettings.parse(listOf("-progressive")).progressive)
        assertTrue(KotlinCompilerSettings.parse(listOf("-progressive=true")).progressive)
        assertFalse(KotlinCompilerSettings.parse(listOf("-progressive=false")).progressive)
        assertFalse(KotlinCompilerSettings.parse(listOf("-progressive", "-progressive=false")).progressive)
        assertTrue(KotlinCompilerSettings.parse(listOf("-progressive=maybe")).warnings.isNotEmpty())
    }

    @Test fun apiVersionAboveTheLanguageVersionIsNotApplied() {
        val s = KotlinCompilerSettings.parse(listOf("-language-version=2.1", "-api-version=2.3"))
        assertEquals("2.1", s.languageVersion)
        assertNull(s.apiVersion)
        assertTrue(s.warnings.any { "greater than languageVersion" in it }, s.warnings.toString())
        assertEquals("2.0", KotlinCompilerSettings.parse(listOf("-language-version=2.1", "-api-version=2.0")).apiVersion)
    }

    // Go's MinSupportedKotlinLanguage ("2.0") must be the first level the embedded compiler
    // supports: Go drops older levels so it does not apply or hash what this helper would drop.
    @Test fun goMinimumSupportedLanguageMatchesTheEmbeddedCompiler() {
        assertTrue(LanguageVersion.KOTLIN_1_9.isUnsupported)
        assertFalse(LanguageVersion.KOTLIN_2_0.isUnsupported)
        assertEquals("2.0", LanguageVersion.entries.first { !it.isUnsupported }.versionString)
    }

    @Test fun requestsCarryKotlinArgs() {
        val request = parseRequest("""{"id":1,"command":"check","files":[],"kotlinArgs":["-language-version=2.1","-opt-in=a.A"],"ruleConfigs":{"R":{"kotlinArgs":["-evil"]}}}""")
        assertEquals(listOf("-language-version=2.1", "-opt-in=a.A"), request.kotlinArgs)
        assertTrue(parseRequest("""{"id":2,"command":"check","files":[]}""").kotlinArgs.isEmpty())
        val modules = parseModuleRequest(
            """{"id":3,"command":"analyzeModules","kotlinArgs":["-progressive"],"modules":[{"id":"m","sourceRoots":[],"generatedSourceRoots":[],"classpath":[],"dependsOn":[],"friends":[],"compilerArgs":["-opt-in=x.Y"]}],"checkFiles":[]}""",
        )!!
        assertEquals(listOf("-progressive"), modules.kotlinArgs)
        assertEquals(listOf("-opt-in=x.Y"), modules.modules.single().compilerArgs)
        assertFalse(modules.modules.single().compilerArgs.isEmpty())
    }
}
