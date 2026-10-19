package dev.jasonpearson.krit.fir

import org.junit.jupiter.api.Test
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertTrue

/**
 * Keeps [FirRule.requiresWholeCompilation] honest. A rule whose source reaches
 * compilation-wide file state cannot give a correct verdict on a slice of a
 * module (and the Analysis API throws on `firProvider` file iteration), so the
 * incremental runner must skip it. Scans the checker sources on disk.
 */
class FirRuleWholeCompilationGateTest {
    private val checkersDir = File("src/main/kotlin/dev/jasonpearson/krit/fir/checkers")

    // Per-symbol `symbolProvider` lookups are fine; these are not.
    private val wholeCompilationUse =
        Regex("""\b(firProvider|getFirFile\w*|getPackageNames|symbolNamesProvider)\b""")
    private val ruleIdDecl = Regex("""\bval\s+ruleId\s*(?::\s*String\s*)?=\s*"([^"]+)"""")

    @Test fun flagMatchesSourceUsage() {
        assertTrue(checkersDir.isDirectory, "run from tools/krit-fir; missing ${checkersDir.absolutePath}")
        val flagged = FirRuleDiscovery.rules.associate { it.ruleId to it.requiresWholeCompilation }
        val problems = mutableListOf<String>()
        for (file in checkersDir.walkTopDown().filter { it.isFile && it.extension == "kt" }.sortedBy { it.path }) {
            val source = stripComments(file.readText())
            val uses = wholeCompilationUse.findAll(source).map { it.value }.toSortedSet()
            val ids = ruleIdDecl.findAll(source).map { it.groupValues[1] }.toList()
            if (ids.isEmpty()) {
                if (uses.isNotEmpty()) {
                    problems += "${file.path} uses $uses but declares no rule id. Move the iteration into the " +
                        "rule's own file (and set requiresWholeCompilation there) or stop iterating the compilation."
                }
                continue
            }
            for (id in ids) {
                val isFlagged = flagged[id] ?: continue
                if (uses.isNotEmpty() && !isFlagged) {
                    problems += "$id (${file.path}) uses $uses, which depends on files outside the set being " +
                        "checked. Add `override val requiresWholeCompilation: Boolean get() = true` to the rule " +
                        "with a comment saying why, or stop using compilation-wide file state."
                }
                if (uses.isEmpty() && isFlagged) {
                    problems += "$id (${file.path}) sets requiresWholeCompilation but no longer uses " +
                        "compilation-wide file state. Remove the override so incremental runs check it."
                }
            }
        }
        assertTrue(problems.isEmpty(), problems.joinToString("\n"))
    }

    @Test fun accessorListsExactlyTheFlaggedRules() {
        val ids = FirRuleDiscovery.wholeCompilationRuleIds()
        assertEquals(FirRuleDiscovery.rules.filter { it.requiresWholeCompilation }.map { it.ruleId }.sorted(), ids)
        assertTrue("SerialVersionUIDInSerializableClass" in ids, ids.toString())
    }

    @Test fun ruleWithoutOverrideDefaultsToFalse() {
        val plain = object : FirRule { override val ruleId = "Plain" }
        assertFalse(plain.requiresWholeCompilation)
        assertFalse(FirRuleDiscovery.rules.first { it.ruleId == "InjectDispatcher" }.requiresWholeCompilation)
    }

    @Test fun scannerIgnoresCommentsAndFlagsRealUse() {
        val src = "// firProvider\n/* getFirFilesByPackage */\nval a = session.firProvider\n"
        assertEquals(listOf("firProvider"), wholeCompilationUse.findAll(stripComments(src)).map { it.value }.toList())
    }

    private fun stripComments(source: String): String =
        source.replace(Regex("""/\*[\s\S]*?\*/"""), "").replace(Regex("""//[^\n]*"""), "")
}
