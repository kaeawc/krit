package dev.jasonpearson.krit.fir

import org.junit.jupiter.api.Test
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Keeps [FirRule.wholeCompilationReason] honest, in both directions. A rule whose source reads facts of other
 * files cannot give a correct verdict on a slice of a module (the rest of the module is binary), and the incremental
 * runner skips tiered rules. Scans the checker sources on disk.
 *
 * What counts as a read of another file:
 *  - compilation-wide file state: `firProvider`, `getFirFile*`, package enumeration;
 *  - another declaration's body or initializer: `resolvedInitializer`, `fir.body`, `fir.initializer`;
 *  - `symbol.source` and `getContainingFile*`, which exist only for a declaration compiled from source.
 *
 * Two-way: a rule with such a read and no reason fails, and a rule with a reason but no read fails. A read that
 * is known not to cross files is listed in [allowedReads] with its proof, and then does not count.
 * `FirIncrementalSplitDifferentialTest` checks the tiering against real slice compiles.
 */
class FirRuleWholeCompilationGateTest {
    private val checkersDir = File("src/main/kotlin/dev/jasonpearson/krit/fir/checkers")

    private val ruleIdDecl = Regex("""\bval\s+ruleId\s*(?::\s*String\s*)?=\s*"([^"]+)"""")

    // Per-symbol `symbolProvider` lookups are fine (class and signature metadata survive in a snapshot); these are not.
    private val reads = listOf(
        Regex("""\b(firProvider|getFirFile\w*|getPackageNames|symbolNamesProvider)\b"""),
        Regex("""\bresolvedInitializer\b"""),
        Regex("""\bfir\??\.(body|initializer)\b"""),
        Regex("""\b(symbol|\w*Symbol)\.source\b"""),
        Regex("""\bgetContainingFile\w*\b"""),
    )

    private enum class Why { LOCAL, CONST, UNVERIFIED }

    /** A source line that matches a read pattern but is excluded from the gate. [line] is matched trimmed and exact. */
    private data class AllowedRead(val ruleId: String, val line: String, val why: Why, val note: String)

    private val allowedReads = listOf(
        // Provably local: the symbol is a local variable of the function under check, so its initializer is in this file.
        AllowedRead("Wakelock", "return assignment?.rValue ?: symbol.resolvedInitializer", Why.LOCAL,
            "valueAt is reached only for FirPropertySymbol.isLocal (the `symbol.isLocal` branch at its call site)"),
        AllowedRead("Wakelock", "var initializer = property.resolvedInitializer ?: return false", Why.LOCAL,
            "the property's source range must lie inside the function being checked (declaredFromNewWakeLock)"),
        AllowedRead("BufferedReadWithoutBuffer", "val initializer = symbol.resolvedInitializer", Why.LOCAL,
            "sourceOfLocal returns early unless symbol.isLocal"),
        AllowedRead("BufferedReadWithoutBuffer", "val declaration = symbol.source?.let { Span(it.startOffset, it.endOffset) }", Why.LOCAL,
            "reaching() is called only from sourceOfLocal, with a local symbol"),
        AllowedRead("GetSignatures", "?: (listOfNotNull(symbol.resolvedInitializer) + assignedValues(symbol, rootFir))", Why.LOCAL,
            "the localVar branch: symbol.isLocal && !symbol.isVal"),
        // Const: the compiler keeps a const property's value in class-file metadata, so a snapshot carries it.
        AllowedRead("GetSignatures", "is FirPropertySymbol -> if (symbol.isConst) symbol.resolvedInitializer else null", Why.CONST,
            "const values are in the metadata"),
        // Reads of a possibly non-local declaration that the split differential did not flag. Not proven local or const:
        // each is a candidate to tier if a cross-file case ever shows a difference (findings doc, section 8.2).
        AllowedRead("SecureRandom", "if (!constructsSecureRandom(property.resolvedInitializer)) return false", Why.UNVERIFIED,
            "initializer of a possibly top-level SecureRandom property"),
        AllowedRead("ShowToast", "val initializer = variable.resolvedInitializer", Why.UNVERIFIED,
            "onlyStore checks that a non-local property is never given a value"),
        AllowedRead("GetSignatures", "is FirFieldSymbol -> if (symbol.fir.isVal) symbol.fir.initializer else null", Why.UNVERIFIED,
            "initializer of a Java field"),
        AllowedRead("GetSignatures", "is FirPropertySymbol -> original.resolvedInitializer", Why.UNVERIFIED,
            "value of a property named GET_SIGNATURES"),
        AllowedRead("GetSignatures", "is FirFieldSymbol -> original.fir.initializer", Why.UNVERIFIED,
            "initializer of a Java field named GET_SIGNATURES"),
        AllowedRead("GetSignatures", "if (!localVar) return symbol.resolvedInitializer?.let { carriesFlag(it, seen) } == true", Why.UNVERIFIED,
            "initializer of a val that may be top-level"),
        AllowedRead("TooGenericExceptionThrown", "val source = symbol.source ?: return false", Why.UNVERIFIED,
            "origin check (is the exception class declared in Kotlin source of this compilation), not a body read"),
        AllowedRead("GlobalScopeLaunchInViewModel", "val initializer = symbol.resolvedInitializer ?: return null", Why.UNVERIFIED,
            "initializer of a final val that may be top-level"),
    )

    private class Hit(val file: File, val line: String)

    @Test fun flagMatchesSourceUsage() {
        assertTrue(checkersDir.isDirectory, "run from tools/krit-fir; missing ${checkersDir.absolutePath}")
        val reasons = FirRuleDiscovery.rules.associate { it.ruleId to it.wholeCompilationReason }
        val problems = mutableListOf<String>()
        val hitsByRule = HashMap<String, List<Hit>>()
        for (file in checkersDir.walkTopDown().filter { it.isFile && it.extension == "kt" }.sortedBy { it.path }) {
            val source = stripComments(file.readText())
            val hits = readLines(source).map { Hit(file, it) }
            val ids = ruleIdDecl.findAll(source).map { it.groupValues[1] }.toList()
            if (ids.isEmpty()) {
                if (hits.isNotEmpty()) {
                    problems += "${file.path} reads other-file state (${hits.joinToString { it.line.take(60) }}) but " +
                        "declares no rule id. Move the read into the rule's own file (and set " +
                        "wholeCompilationReason there) or stop reading other declarations."
                }
                continue
            }
            for (id in ids) hitsByRule[id] = hits
        }
        for ((id, hits) in hitsByRule.toSortedMap()) {
            if (id !in reasons) continue
            val allowed = allowedReads.filter { it.ruleId == id }
            val counted = hits.filter { h -> allowed.none { it.line == h.line } }
            val reason = reasons[id]
            if (counted.isNotEmpty() && reason == null) {
                problems += "$id (${counted.first().file.path}) reads facts of other declarations or files: " +
                    counted.joinToString("; ") { "`${it.line.take(90)}`" } + ". A slice compile reads a declaration " +
                    "from a binary snapshot without its body or initializer, so the verdict changes. Add " +
                    "`override val wholeCompilationReason = \"<the fact it reads>\"` to the rule. If the read is " +
                    "provably local (a local variable) or const, add it to allowedReads in this test with the proof."
            }
            if (counted.isEmpty() && reason != null) {
                problems += "$id sets wholeCompilationReason but no longer reads other declarations' bodies or " +
                    "initializers or the compilation's files. Remove the override so incremental runs check it, " +
                    "or, if the read is still there but provably local, list it in allowedReads."
            }
        }
        for (a in allowedReads) {
            val hits = hitsByRule[a.ruleId]
            if (hits == null || hits.none { it.line == a.line }) {
                problems += "allowedReads entry for ${a.ruleId} matches no read line: `${a.line}`. The code moved " +
                    "or changed; update or delete the entry (and re-check that the read is still ${a.why.name.lowercase()})."
            }
            if (a.ruleId in reasons && reasons[a.ruleId] != null) {
                problems += "allowedReads lists ${a.ruleId}, which is tiered. Delete the entry; tiered rules " +
                    "need no allowance."
            }
        }
        assertTrue(problems.isEmpty(), problems.joinToString("\n"))
    }

    @Test fun everyTieredRuleStatesAReason() {
        val tiered = FirRuleDiscovery.wholeCompilationReasons()
        assertEquals(FirRuleDiscovery.wholeCompilationRuleIds(), tiered.keys.toList())
        for ((id, reason) in tiered) assertTrue(reason.isNotBlank() && reason.length > 20, "$id needs a real reason: '$reason'")
    }

    @Test fun theRulesTheSplitDifferentialFoundAreTiered() {
        // Findings doc, section 8.1: rules whose verdict changed when other files were binaries. More may be added.
        val found = listOf(
            "CollectionsSynchronizedListIteration", "HardcodedSecretKey", "JdbcStatementExecute",
            "LocaleGetDefaultForFormatting", "LongLogTag", "SerialVersionUIDInSerializableClass",
            "SharedFlowWithoutReplay", "ShortAlarm", "StaticIv", "WeakKeySize", "WorldWriteableFiles",
        )
        val tiered = FirRuleDiscovery.wholeCompilationRuleIds()
        assertTrue(tiered.containsAll(found), "missing: ${found - tiered.toSet()}")
    }

    @Test fun ruleWithoutOverrideIsNotTiered() {
        val plain = object : FirRule { override val ruleId = "Plain" }
        assertNull(plain.wholeCompilationReason)
        assertFalse(plain.requiresWholeCompilation)
        assertFalse(FirRuleDiscovery.rules.first { it.ruleId == "InjectDispatcher" }.requiresWholeCompilation)
        val tiered = FirRuleDiscovery.rules.first { it.ruleId == "SerialVersionUIDInSerializableClass" }
        assertTrue(tiered.requiresWholeCompilation)
        assertNotNull(tiered.wholeCompilationReason)
    }

    @Test fun scannerIgnoresCommentsAndFlagsRealReads() {
        val src = """
            // firProvider
            /* getFirFilesByPackage, resolvedInitializer */
            val a = session.firProvider
            val b = symbol.resolvedInitializer
            val c = getter.fir.body
            val d = klass.fir?.initializer
            val e = functionSymbol.source
            val f = call.source
            val g = containingFileSymbol?.fir?.imports
            val h = symbol.resolvedStatus
        """.trimIndent()
        assertEquals(
            listOf(
                "val a = session.firProvider", "val b = symbol.resolvedInitializer", "val c = getter.fir.body",
                "val d = klass.fir?.initializer", "val e = functionSymbol.source",
            ),
            readLines(stripComments(src)),
        )
    }

    private fun readLines(source: String): List<String> =
        source.lineSequence().map { it.trim() }.filter { line -> line.isNotEmpty() && reads.any { it.containsMatchIn(line) } }
            .filterNot { it.startsWith("import ") }.toList()

    private fun stripComments(source: String): String =
        source.replace(Regex("""/\*[\s\S]*?\*/"""), "").replace(Regex("""//[^\n]*"""), "")
}
