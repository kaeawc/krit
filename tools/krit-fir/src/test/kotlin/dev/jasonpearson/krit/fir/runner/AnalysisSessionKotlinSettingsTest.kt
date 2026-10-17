package dev.jasonpearson.krit.fir.runner

import dev.jasonpearson.krit.fir.RequestResult
import dev.jasonpearson.krit.fir.handleRequestLine
import dev.jasonpearson.krit.fir.jsonStr
import dev.jasonpearson.krit.fir.parseRequest
import dev.jasonpearson.krit.fir.sessionNeedsRebuild
import dev.jasonpearson.krit.fir.withWarnings
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.io.File
import java.nio.file.Path
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotEquals
import kotlin.test.assertTrue

/**
 * The same source analyzed under different project Kotlin settings must give
 * the verdict that project's compiler would: a newer embedded compiler may not
 * silently change the language semantics of a project pinned to an older level.
 */
class AnalysisSessionKotlinSettingsTest {
    @TempDir lateinit var tmp: Path

    private val stdlib = File(Unit::class.java.protectionDomain.codeSource.location.toURI()).absolutePath
    private var counter = 0

    private fun sourceDir(code: String): File =
        tmp.resolve("src-${counter++}").toFile().apply { mkdirs(); File(this, "A.kt").writeText(code) }

    private fun session(dir: File, kotlinArgs: List<String>) = AnalysisSession(listOf(dir.path), listOf(stdlib), "", kotlinArgs)

    private fun errorsFor(code: String, kotlinArgs: List<String>): Map<String, String> {
        val dir = sourceDir(code)
        val s = session(dir, kotlinArgs)
        try {
            return s.check(1, listOf(FileRef(File(dir, "A.kt").path)), setOf("ProtocolProbe")).errorFiles.mapKeys { File(it.key).name }
        } finally { s.dispose() }
    }

    private fun propertyAnnotations(kotlinArgs: List<String>): List<String> {
        val s = session(sourceDir(ANNOTATION_TARGET), kotlinArgs)
        try {
            val box = s.analyze(emptyList()).dependencies["p.Box"]!!
            return box.members.single { it.kind == "property" && it.name == "label" }.annotations
        } finally { s.dispose() }
    }

    // Annotation-target default: with the 2.4 default an annotation on a constructor
    // `val` applies to the parameter and the property; under the older language
    // levels it applies to the first applicable target only, the parameter.
    @Test fun annotationTargetDefaultFollowsTheProjectsLanguageLevel() {
        assertEquals(listOf("p.Tag"), propertyAnnotations(emptyList()), "embedded 2.4 default")
        for (level in listOf("2.1", "2.2", "2.3")) {
            assertEquals(emptyList(), propertyAnnotations(listOf("-language-version=$level", "-api-version=$level")),
                "a project pinned to $level keeps first-only annotation targets")
        }
        assertEquals(emptyList(), propertyAnnotations(listOf("-Xannotation-default-target=first-only")))
        assertEquals(listOf("p.Tag"), propertyAnnotations(listOf("-language-version=2.3", "-api-version=2.3",
            "-Xannotation-default-target=param-property")), "the flag re-enables the new behavior on the old level")
    }

    @Test fun languageFeaturesFollowTheProjectsLanguageLevelAndFlags() {
        assertEquals(emptyMap(), errorsFor(WHEN_GUARD, emptyList()))
        val older = errorsFor(WHEN_GUARD, listOf("-language-version=2.1", "-api-version=2.1"))
        assertEquals(setOf("A.kt"), older.keys)
        assertTrue("when guards" in older.getValue("A.kt"), older.toString())
        assertEquals(emptyMap(), errorsFor(WHEN_GUARD, listOf("-language-version=2.1", "-api-version=2.1", "-Xwhen-guards")))
        assertEquals(emptyMap(), errorsFor(WHEN_GUARD, listOf("-language-version=2.1", "-api-version=2.1", "-XXLanguage:+WhenGuards")))
    }

    @Test fun optInMarkersAreAppliedFromTheProjectSettings() {
        assertEquals(setOf("A.kt"), errorsFor(OPT_IN, emptyList()).keys)
        assertEquals(emptyMap(), errorsFor(OPT_IN, listOf("-opt-in=p.Experimental")))
    }

    @Test fun unsupportedLanguageVersionIsReportedAndTheEmbeddedDefaultApplies() {
        val dir = sourceDir(WHEN_GUARD)
        val s = session(dir, listOf("-language-version=1.9", "-api-version=1.9"))
        try {
            assertTrue(s.kotlinSettings.isEmpty, s.kotlinSettings.toString())
            val warnings = s.kotlinSettingsWarnings.joinToString("\n")
            assertTrue("languageVersion 1.9 is not supported" in warnings && "apiVersion 1.9 is not supported" in warnings, warnings)
            // The unusable API level is dropped with it; the compiler would otherwise reject the whole compile.
            assertEquals(emptyMap(), s.check(1, listOf(FileRef(File(dir, "A.kt").path)), setOf("ProtocolProbe")).errorFiles)
        } finally { s.dispose() }
    }

    @Test fun kritOwnedArgumentsAndUnsupportedFlagsAreReportedNotApplied() {
        val dir = sourceDir(WHEN_GUARD)
        val s = session(dir, listOf("-d", "/nonexistent/out", "-classpath=/evil.jar", "-Xplugin=/evil-plugin.jar",
            "-Xfriend-paths=/x", "-Xfragments=a,b", "-module-name=evil", "-Werror", "-Xcontext-parameters"))
        try {
            assertEquals(listOf("-Xcontext-parameters"), s.kotlinSettings.flags)
            val warnings = s.kotlinSettingsWarnings
            for (arg in listOf("-d /nonexistent/out", "-classpath=/evil.jar", "-Xplugin=/evil-plugin.jar", "-Xfriend-paths=/x",
                "-Xfragments=a,b", "-module-name=evil")) {
                assertTrue(warnings.any { arg in it && "managed by krit" in it }, "$arg not reported: $warnings")
            }
            assertTrue(warnings.any { "-Werror" in it && "not an analysis-semantic" in it }, warnings.toString())
            // Analysis is unaffected by the ignored arguments: resolution still sees the stdlib and sources.
            assertEquals(emptyMap(), s.check(1, listOf(FileRef(File(dir, "A.kt").path)), setOf("ProtocolProbe")).errorFiles)
        } finally { s.dispose() }
    }

    // Cold, warm and rebuilt-session results under the same settings equal a fresh run.
    @Test fun daemonResultsMatchAFreshRunWithTheSameSettings() {
        val dir = sourceDir(WHEN_GUARD)
        val file = File(dir, "A.kt").path
        val older = listOf("-language-version=2.1", "-api-version=2.1")

        fun request(id: Int, args: List<String>?): String =
            """{"id":$id,"command":"check","files":[{"path":${jsonStr(file)}}],"sourceDirs":[${jsonStr(dir.path)}],""" +
                """"classpath":[${jsonStr(stdlib)}],"rules":["ProtocolProbe"]""" +
                (args?.let { ""","kotlinArgs":[${it.joinToString(",") { a -> jsonStr(a) }}]""" } ?: "") + "}"

        fun errorFiles(json: String): String = Regex(""""errorFiles":(\{[^}]*\})""").find(json)!!.groupValues[1]

        val daemon = AnalysisSession(listOf(dir.path), listOf(stdlib))
        var current = daemon
        try {
            fun send(line: String): String {
                val result = handleRequestLine(line, current, System.currentTimeMillis())
                return when (result) {
                    is RequestResult.Response -> result.json
                    is RequestResult.SessionRebuilt -> { current = result.newSession; result.json }
                    else -> error(result.toString())
                }
            }
            val cold = errorFiles(send(request(1, older)))
            val warm = errorFiles(send(request(2, older)))
            val unchanged = errorFiles(send(request(3, null)))
            assertNotEquals("{}", cold, "the older level must gate the file")
            assertEquals(cold, warm)
            assertEquals(cold, unchanged, "a request that names no settings keeps the session's")

            val fresh = AnalysisSession(listOf(dir.path), listOf(stdlib), "", older)
            try {
                assertEquals(cold, errorFiles(buildFreshResponse(fresh, file)))
            } finally { fresh.dispose() }

            // Changing only the settings rebuilds the session and changes the verdict.
            val modern = errorFiles(send(request(4, listOf("-language-version=2.4", "-api-version=2.4"))))
            assertEquals("{}", modern)
            assertFalse(current === daemon, "a settings-only change must rebuild the session")
        } finally {
            current.dispose()
            if (current !== daemon) daemon.dispose()
        }
    }

    private fun buildFreshResponse(session: AnalysisSession, file: String): String =
        dev.jasonpearson.krit.fir.buildCheckResponse(session.check(9, listOf(FileRef(file)), setOf("ProtocolProbe")))

    @Test fun sessionRebuildsOnlyForAnEffectiveSettingsChange() {
        val session = AnalysisSession(listOf("/src"), listOf("/cp.jar"), "", listOf("-language-version=2.1", "-opt-in=b.B", "-opt-in=a.A"))
        fun needs(args: String) = sessionNeedsRebuild(parseRequest("""{"id":1,"command":"check","files":[]$args}"""), session)
        assertFalse(needs(""), "no settings named: keep the session's")
        assertFalse(needs(""","kotlinArgs":["-opt-in=a.A","-language-version=2.1","-opt-in=b.B"]"""), "reordering is not a change")
        assertFalse(needs(""","kotlinArgs":["-language-version=2.1","-opt-in=a.A","-opt-in=b.B","-Werror"]"""), "a rejected extra is not a change")
        assertTrue(needs(""","kotlinArgs":["-language-version=2.2","-opt-in=a.A","-opt-in=b.B"]"""))
        assertTrue(needs(""","kotlinArgs":["-language-version=2.1","-opt-in=a.A"]"""))
        assertTrue(needs(""","kotlinArgs":["-language-version=2.1","-opt-in=a.A","-opt-in=b.B","-Xwhen-guards"]"""))
        session.dispose()
    }

    @Test fun warningsAreAppendedToTheResponse() {
        val session = AnalysisSession(emptyList(), emptyList(), "", listOf("-Werror", "-language-version=1.9"))
        try {
            val json = withWarnings("""{"id":1}""", session, "")
            assertTrue(json.startsWith("""{"id":1,"warning":"""), json)
            assertTrue("-Werror" in json && "languageVersion 1.9 is not supported" in json, json)
            assertEquals("""{"id":1}""", withWarnings("""{"id":1}""", AnalysisSession(emptyList(), emptyList()), ""))
        } finally { session.dispose() }
    }

    private companion object {
        val WHEN_GUARD = """
            package p
            fun f(x: Any): Int = when (x) {
                is String if x.isNotEmpty() -> 1
                else -> 0
            }
        """.trimIndent()

        val ANNOTATION_TARGET = """
            package p
            @Target(AnnotationTarget.VALUE_PARAMETER, AnnotationTarget.PROPERTY)
            annotation class Tag
            class Box(@Tag val label: String)
        """.trimIndent()

        val OPT_IN = """
            package p
            @RequiresOptIn(level = RequiresOptIn.Level.ERROR)
            annotation class Experimental
            @Experimental fun api() = 1
            fun use() = api()
        """.trimIndent()
    }
}
