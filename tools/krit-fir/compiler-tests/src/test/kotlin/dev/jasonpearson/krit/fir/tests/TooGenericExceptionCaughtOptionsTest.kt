package dev.jasonpearson.krit.fir.tests

import dev.jasonpearson.krit.fir.FirRuleCompileContext
import java.io.File
import org.junit.jupiter.api.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

// TooGenericExceptionCaught options: the generic names and the allowed variable-name pattern.
class TooGenericExceptionCaughtOptionsTest {

    private val source = """
        package caughtoptions

        import java.io.IOException

        fun f() {
            try { println() } catch (e: Exception) { println(e) }
            try { println() } catch (ignored: Exception) { println(ignored) }
            try { println() } catch (e: IOException) { println(e) }
            try { println() } catch (ignoredIo: IOException) { println(ignoredIo) }
            try { println() } catch (tmp: IllegalStateException) { println(tmp) }
        }
    """.trimIndent()

    private fun reported(options: Map<String, Any?> = emptyMap(), sourceText: String = source): List<Int> {
        val result = KritFirProbe.compile(
            mapOf("F.kt" to sourceText),
            FirRuleCompileContext(
                enabledRuleIds = setOf("TooGenericExceptionCaught"),
                ruleConfigs = mapOf("TooGenericExceptionCaught" to options),
            ),
        )
        assertTrue(result.clean, result.problems())
        return result.diags.filter { it.name == "TooGenericExceptionCaught" && it.file == "F.kt" }.map { it.line }.sorted()
    }

    @Test fun defaultsFlagOnlyTheGenericException() = assertEquals(listOf(6), reported())

    // A configured name matches a caught class of that simple name in any package.
    @Test fun configuredNamesReplaceTheDefaults() {
        assertEquals(listOf(8), reported(mapOf("exceptionNames" to listOf("IOException"))))
        assertEquals(listOf(6, 8, 10), reported(mapOf("exceptionNames" to listOf("Exception", "IOException", "IllegalStateException"))))
    }

    // An empty list falls back to the default names, as Go does.
    @Test fun emptyNamesFallBackToDefaults() = assertEquals(listOf(6), reported(mapOf("exceptionNames" to emptyList<String>())))

    // A configured pattern replaces the default and is a full match.
    @Test fun allowedPatternIsFullMatch() {
        val names = listOf("IOException", "IllegalStateException")
        assertEquals(listOf(8, 9), reported(mapOf("exceptionNames" to names, "allowedExceptionNameRegex" to "tmp.*")))
        assertEquals(listOf(8, 9, 10), reported(mapOf("exceptionNames" to names, "allowedExceptionNameRegex" to "tm")))
    }

    @Test fun validAllowedPatternIsHonored() {
        assertEquals(listOf(6), reported(mapOf("allowedExceptionNameRegex" to "ignored")))
    }

    // The Go side reads the same fixtures (internal/firchecks/rule_configs_test.go) and asserts their
    // ruleConfigs; this reads that expected FIR input so both sides stay pinned to one file.
    private fun normalizedOptions(name: String): Map<String, Any?> {
        val json = File("src/test/data/options/TooGenericExceptionCaught/$name.json").readText()
        val ruleConfigs = json.substringAfter("\"ruleConfigs\"")
        check("\"exceptionNames\": [\"Exception\"]" in ruleConfigs && "allowedExceptionNameRegex" !in ruleConfigs) { json }
        return mapOf("exceptionNames" to listOf("Exception"))
    }

    @Test fun blankAllowedPatternUsesDefaultAfterGoNormalization() {
        assertEquals(listOf(6), reported(normalizedOptions("blank")))
    }

    @Test fun lookaheadAllowedPatternUsesDefaultAfterGoNormalization() {
        val sourceText = "fun f() { try { println() } catch (tmp: Exception) { println(tmp) } }"
        assertEquals(listOf(1), reported(normalizedOptions("lookahead"), sourceText))
    }

    @Test fun posixClassAllowedPatternIsPinnedConservativeMiss() {
        assertEquals(emptyList(), reported(mapOf("allowedExceptionNameRegex" to "[[:alpha:]]+")))
    }

    @Test fun unicodeCaseInsensitiveAllowedPatternIsPinnedConservativeMiss() {
        val unicodeSource = "fun f() { try { println() } catch (Éx: Exception) { println(Éx) } }"
        assertEquals(emptyList(), reported(mapOf("allowedExceptionNameRegex" to "(?i)é.*"), unicodeSource))
    }

    @Test fun octalEscapeAllowedPatternIsPinnedConservativeMiss() {
        val octalSource = "fun f() { try { println() } catch (Ax: Exception) { println(Ax) } }"
        assertEquals(emptyList(), reported(mapOf("allowedExceptionNameRegex" to "\\101.*"), octalSource))
    }

    @Test fun anchoredAllowedPatternIsPinnedConservativeMiss() {
        val anchoredSource = "fun f() { try { println() } catch (ignoreMe: Exception) { println(ignoreMe) } }"
        assertEquals(emptyList(), reported(mapOf("allowedExceptionNameRegex" to "^ignore|tmp$"), anchoredSource))
    }

    @Test fun plainAllowedPatternsAreHonored() {
        for (pattern in listOf("ign.red", "ignored|tmp", "(ignore)d", "ignore[d]?", "ignore[d]*", "ignore[d]+")) {
            assertEquals(listOf(6), reported(mapOf("allowedExceptionNameRegex" to pattern)), pattern)
        }
        assertEquals(emptyList(), reported(mapOf("allowedExceptionNameRegex" to "[a-z]+")))
        assertEquals(emptyList(), reported(mapOf("allowedExceptionNameRegex" to "\\w+")))
    }

    @Test fun nonPlainAllowedPatternsArePinnedConservativeMisses() {
        for (pattern in listOf("[[a]b]+", "[a[b]]", "[a&&b]+", "\\p{L}+", "\\P{N}+", "\\Qignored\\E", "\\Gignored", "ignored\\Z", "ignored\\z", "ignore\\.d", "e?+", "e*+", "e*?", "e++", "e{1,2}+", "e{1,2}", "(?>e)")) {
            assertEquals(emptyList(), reported(mapOf("allowedExceptionNameRegex" to pattern)), pattern)
        }
    }

    // Java cannot compile this Go-only syntax, so pin the conservative FIR miss.
    @Test fun goOnlyNamedAllowedPatternIsPinnedConservativeMiss() {
        val names = listOf("IOException", "IllegalStateException")
        assertEquals(emptyList(), reported(mapOf("exceptionNames" to names, "allowedExceptionNameRegex" to "(?P<n>tmp).*")))
    }

    @Test fun invalidAllowedPatternUsesDefaultAfterGoNormalization() {
        assertEquals(listOf(6), reported(normalizedOptions("invalid")))
    }
}
