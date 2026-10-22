package dev.jasonpearson.krit.types

import org.jetbrains.kotlin.cli.common.arguments.K2JVMCompilerArguments
import org.jetbrains.kotlin.config.AnalysisFlags
import org.jetbrains.kotlin.config.ExplicitApiMode
import org.jetbrains.kotlin.config.LanguageFeature
import org.jetbrains.kotlin.config.LanguageVersion
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotEquals
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

class KotlinCompilerSettingsTest {
    @Test
    fun parsesVersionsOptInsAndAllowlistedFlags() {
        val s = KotlinCompilerSettings.parse(listOf(
            "-language-version=2.1", "-api-version", "2.0", "-opt-in=b.B", "-Xopt-in=a.A", "-progressive",
            "-Xcontext-parameters", "-XXLanguage:+WhenGuards", "-XXLanguage:-WhenGuards", "-Xjvm-default=all",
        ))
        assertEquals("2.1", s.languageVersion)
        assertEquals("2.0", s.apiVersion)
        assertEquals(listOf("a.A", "b.B"), s.optIns)
        assertTrue(s.progressive)
        assertEquals(listOf("-Xcontext-parameters", "-XXLanguage:-WhenGuards", "-Xjvm-default=all"), s.flags)
        assertTrue(s.warnings.isEmpty(), s.warnings.toString())
        assertEquals(s.copy(warnings = emptyList()), KotlinCompilerSettings.parse(s.arguments).copy(warnings = emptyList()))
    }

    // `@all:` needs -Xannotation-target-all on a project pinned below Kotlin 2.4.
    @Test
    fun annotationTargetAllFlagIsApplied() {
        val s = KotlinCompilerSettings.parse(listOf("-language-version=2.3", "-api-version=2.3", "-Xannotation-target-all"))
        assertEquals(listOf("-Xannotation-target-all"), s.flags)
        assertTrue(s.warnings.isEmpty(), s.warnings.toString())
        assertTrue(KotlinCompilerSettings.parse(listOf("-Xannotation-target-all=true")).warnings.single().contains("takes no value"))
    }

    @Test
    fun protectedArgumentsAndUnsupportedFlagsAreReportedNotApplied() {
        val s = KotlinCompilerSettings.parse(listOf(
            "-d", "/out", "-classpath=/a.jar", "-jvm-target", "1.8", "-module-name=m", "-Xplugin=/p.jar", "-P", "plugin:x:y=z",
            "-Xfriend-paths=/f", "-Xfragments=a,b", "-Werror", "-Xno-param-assertions", "-language-version=1.9", "-api-version=1.9",
        ))
        assertTrue(s.isEmpty, s.toString())
        val warnings = s.warnings.joinToString("\n")
        for (managed in listOf("-d /out", "-classpath=/a.jar", "-jvm-target 1.8", "-module-name=m", "-Xplugin=/p.jar",
            "-P plugin:x:y=z", "-Xfriend-paths=/f", "-Xfragments=a,b")) {
            assertTrue("$managed (managed by krit)" in warnings, "$managed: $warnings")
        }
        assertTrue("-Werror (not an analysis-semantic" in warnings && "-Xno-param-assertions" in warnings, warnings)
        assertTrue("languageVersion 1.9 is not supported" in warnings && "apiVersion 1.9 is not supported" in warnings, warnings)
    }

    @Test
    fun identityChangesWithEverySemanticSetting() {
        val base = KotlinCompilerSettings.parse(listOf("-language-version=2.1", "-api-version=2.1", "-opt-in=a.A", "-Xcontext-parameters"))
        assertEquals(base.identity, KotlinCompilerSettings.parse(listOf("-Xcontext-parameters", "-opt-in=a.A", "-api-version=2.1",
            "-language-version=2.1", "-opt-in=a.A", "-Werror")).identity)
        for (variant in listOf(
            listOf("-language-version=2.2", "-api-version=2.1", "-opt-in=a.A", "-Xcontext-parameters"),
            listOf("-language-version=2.1", "-api-version=2.0", "-opt-in=a.A", "-Xcontext-parameters"),
            listOf("-language-version=2.1", "-api-version=2.1", "-opt-in=a.A", "-opt-in=b.B", "-Xcontext-parameters"),
            listOf("-language-version=2.1", "-api-version=2.1", "-opt-in=a.A"),
        )) assertNotEquals(base.identity, KotlinCompilerSettings.parse(variant).identity, variant.toString())
        assertEquals("", KotlinCompilerSettings().identity)
    }

    @Test
    fun nullabilityAnnotationValuesMayContainAtSigns() {
        val s = KotlinCompilerSettings.parse(listOf(
            "-Xnullability-annotations=@org.jspecify.annotations:strict", "-Xjsr305=@org.example.Nullable:warn", "-Xjsr305=strict",
        ))
        assertTrue(s.warnings.isEmpty(), s.warnings.toString())
        assertEquals(listOf("-Xnullability-annotations=@org.jspecify.annotations:strict", "-Xjsr305=strict"), s.flags)
        assertTrue(KotlinCompilerSettings.parse(listOf("@-Xjsr305=strict")).isEmpty)
    }

    @Test
    fun progressiveFalseTurnsProgressiveModeOff() {
        assertTrue(KotlinCompilerSettings.parse(listOf("-progressive=true")).progressive)
        assertFalse(KotlinCompilerSettings.parse(listOf("-progressive=false")).progressive)
        assertFalse(KotlinCompilerSettings.parse(listOf("-progressive", "-progressive=false")).progressive)
        assertTrue(KotlinCompilerSettings.parse(listOf("-progressive=maybe")).warnings.isNotEmpty())
    }

    @Test
    fun apiVersionAboveTheLanguageVersionIsNotApplied() {
        val s = KotlinCompilerSettings.parse(listOf("-language-version=2.1", "-api-version=2.3"))
        assertEquals("2.1", s.languageVersion)
        assertNull(s.apiVersion)
        assertTrue(s.warnings.any { "greater than languageVersion" in it }, s.warnings.toString())
    }

    // Go's MinSupportedKotlinLanguage ("2.0") must be the first level the embedded compiler
    // supports: Go drops older levels so it does not apply or hash what this helper would drop.
    @Test
    fun goMinimumSupportedLanguageMatchesTheEmbeddedCompiler() {
        assertTrue(LanguageVersion.KOTLIN_1_9.isUnsupported)
        assertFalse(LanguageVersion.KOTLIN_2_0.isUnsupported)
        assertEquals("2.0", LanguageVersion.entries.first { !it.isUnsupported }.versionString)
    }

    @Test
    fun noSettingsLeaveTheEmbeddedDefaultsInForce() {
        assertNull(KotlinCompilerSettings().toLanguageVersionSettings())
        assertNull(KotlinCompilerSettings.parse(listOf("-Werror")).toLanguageVersionSettings())
    }

    @Test
    fun analysisApiLanguageSettingsFollowTheProjectSettings() {
        val older = KotlinCompilerSettings.parse(listOf("-language-version=2.1", "-api-version=2.1"))
        val olderSettings = assertNotNull(older.toLanguageVersionSettings())
        assertEquals(LanguageVersion.KOTLIN_2_1, olderSettings.languageVersion)
        assertFalse(olderSettings.supportsFeature(LanguageFeature.WhenGuards), "when guards need 2.2 or the flag")

        val flagged = KotlinCompilerSettings.parse(listOf("-language-version=2.1", "-api-version=2.1", "-Xwhen-guards"))
        assertTrue(assertNotNull(flagged.toLanguageVersionSettings()).supportsFeature(LanguageFeature.WhenGuards))

        val toggled = KotlinCompilerSettings.parse(listOf("-language-version=2.1", "-api-version=2.1", "-XXLanguage:+WhenGuards"))
        assertTrue(assertNotNull(toggled.toLanguageVersionSettings()).supportsFeature(LanguageFeature.WhenGuards))

        val optIn = assertNotNull(KotlinCompilerSettings.parse(listOf("-opt-in=a.A")).toLanguageVersionSettings())
        assertEquals(listOf("a.A"), optIn.getFlag(AnalysisFlags.optIn))
        assertEquals(ExplicitApiMode.DISABLED, optIn.getFlag(AnalysisFlags.explicitApiMode))

        // Without a version setting the language level stays the embedded compiler's.
        assertEquals(LanguageVersion.LATEST_STABLE, optIn.languageVersion)
    }

    @Test
    fun appliesWithoutTouchingOwnedFields() {
        val target = K2JVMCompilerArguments().apply { destination = "/krit/out"; classpath = "/krit/cp"; moduleName = "krit_m" }
        KotlinCompilerSettings.parse(listOf("-d", "/evil", "-classpath=/evil", "-module-name=evil", "-opt-in=a.A")).applyTo(target)
        assertEquals("/krit/out", target.destination)
        assertEquals("/krit/cp", target.classpath)
        assertEquals("krit_m", target.moduleName)
        assertEquals(listOf("a.A"), target.optIn?.toList())
    }

    // Keeps this copy of the allowlist equal to Go's and krit-fir's: the Go test counts the
    // `allowlist:` tags and looks for each quoted flag name in this file.
    @Test
    fun allowlistMatchesTheTaggedSourceLines() {
        val source = File("src/main/kotlin/dev/jasonpearson/krit/types/KotlinCompilerSettings.kt").readText()
        val tagged = source.lines().filter { "// allowlist:" in it }
        assertEquals(KotlinCompilerSettings.ALLOWLIST.size, tagged.size)
        for (name in KotlinCompilerSettings.ALLOWLIST.keys) assertTrue(tagged.any { "\"$name\"" in it }, name)
        assertTrue(KotlinCompilerSettings.ALLOWLIST.keys.none { KotlinCompilerSettings.isProtected(it) })
    }

    @Test
    fun cliCollectsRepeatedKotlinArgs() {
        val args = parseArgs(arrayOf("--sources", "/src", "--kotlin-arg", "-language-version=2.1", "--kotlin-arg", "-opt-in=a.A"))
        assertEquals(listOf("-language-version=2.1", "-opt-in=a.A"), assertNotNull(args).kotlinArgs)
        assertTrue(assertNotNull(parseArgs(arrayOf("--sources", "/src"))).kotlinArgs.isEmpty())
        assertNull(parseArgs(arrayOf("--sources", "/src", "--kotlin-arg")))
    }
}
