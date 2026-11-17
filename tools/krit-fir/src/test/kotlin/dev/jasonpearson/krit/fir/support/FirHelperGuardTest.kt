package dev.jasonpearson.krit.fir.support

import dev.jasonpearson.krit.fir.stripCommentsAndStrings
import java.io.File
import kotlin.test.Test
import kotlin.test.assertEquals
import kotlin.test.assertTrue

/**
 * Keeps FIR checkers on the shared helpers in `support/`, so a compiler API change lands in one place.
 *
 * Scans the checker sources on disk for compiler-internal patterns that a helper already covers. A file may use a
 * pattern only as often as `fir-helper-guard-allowlist.txt` records, and the allowlist must match exactly: a new use
 * fails, and a migrated use fails until its entry is lowered. The allowlist only shrinks. The policy is in
 * docs/fir-checker-authoring.md ("Use the shared helpers").
 */
class FirHelperGuardTest {
    private val checkersDir = File("src/main/kotlin/dev/jasonpearson/krit/fir/checkers")
    private val supportDir = File("src/main/kotlin/dev/jasonpearson/krit/fir/support")
    private val allowlistFile = File("src/test/resources/fir-helper-guard-allowlist.txt")

    @Test fun checkersUseTheSharedHelpers() {
        assertTrue(checkersDir.isDirectory, "run from tools/krit-fir; missing ${checkersDir.absolutePath}")
        val allowed = readAllowlist()
        val actual = scanCheckers()
        val problems = mutableListOf<String>()
        for (key in (allowed.keys + actual.keys).sorted()) {
            val pattern = helperPatterns.first { it.id == key.pattern }
            val have = actual[key] ?: 0
            val limit = allowed[key] ?: 0
            if (have > limit) {
                problems += "${key.file} uses ${pattern.directApi} $have time(s), allowlist $limit. Use " +
                    "${pattern.helpers} from support/ instead. If no helper fits, add one to support/ first " +
                    "(docs/fir-checker-authoring.md, \"Use the shared helpers\")."
            } else if (have < limit) {
                val fix = if (have == 0) "delete the entry" else "lower the entry to $have"
                problems += "${key.file} uses ${pattern.directApi} $have time(s), allowlist $limit: $fix " +
                    "in ${allowlistFile.path}. The allowlist only shrinks."
            }
        }
        assertTrue(
            problems.isEmpty(),
            problems.joinToString("\n") + "\n\nCurrent uses:\n" +
                actual.toSortedMap().entries.joinToString("\n") { (k, n) -> "${k.file} ${k.pattern} $n" },
        )
    }

    @Test fun allowlistIsSortedAndUnique() {
        val keys = allowlistLines().map { it.split(Regex("\\s+")).take(2).joinToString(" ") }
        assertEquals(keys.sorted().distinct(), keys, "keep ${allowlistFile.path} sorted, one line per file and pattern")
    }

    @Test fun everyNamedHelperExists() {
        val declared = supportDir.walkTopDown().filter { it.isFile && it.extension == "kt" }
            .flatMap { Regex("""\bfun\s+(?:[A-Za-z.]+\.)?(\w+)\s*\(""").findAll(it.readText()).map { m -> m.groupValues[1] } }
            .toSet()
        val missing = helperPatterns.flatMap { it.helpers.split(" / ") }.map { it.substringBefore('(') }
            .filter { it !in declared }
        assertEquals(emptyList(), missing, "helper patterns name helpers that support/ does not declare")
    }

    @Test fun scannerCountsCodeOnly() {
        val src = """
            import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
            // ClassId(FqName("a"), Name.identifier("B"))
            /* lookupSuperTypes(symbol) */
            val a = ClassId(FqName("a"), Name.identifier("B")) to ClassId.topLevel(FqName("a.B"))
            val b = "ClassId(FqName(x))"
            val c = if (e is FirSmartCastExpression) e.originalExpression else e
            val d = (s as FirCheckedSafeCallSubject).originalReceiverRef.value
            val f = if (x is FirWrappedArgumentExpression) x.expression else x
            val g = id.constructClassLikeType(emptyArray(), isMarkedNullable = true)
            val h = (n as? FirCheckNotNullCall)?.argumentList?.arguments?.firstOrNull()
            val i = source.treeStructure.toString(node).toString()
            val j = CallableId(classIdOf("a", "B"), Name.identifier("c"))
        """.trimIndent()
        assertEquals(
            mapOf(
                "class-id-constructor" to 1, "class-id-factory" to 1, "smart-cast-unwrap" to 1,
                "safe-call-subject-unwrap" to 1, "argument-unwrap" to 1, "class-type-constructor" to 1,
                "not-null-unwrap" to 1, "light-text" to 1,
            ),
            countPatterns(src),
        )
    }

    private data class Key(val file: String, val pattern: String) : Comparable<Key> {
        override fun compareTo(other: Key) = compareValuesBy(this, other, Key::file, Key::pattern)
    }

    private fun scanCheckers(): Map<Key, Int> {
        val out = HashMap<Key, Int>()
        for (file in checkersDir.walkTopDown().filter { it.isFile && it.extension == "kt" }) {
            val relative = file.relativeTo(checkersDir).invariantSeparatorsPath
            for ((id, n) in countPatterns(file.readText())) out[Key(relative, id)] = n
        }
        return out
    }

    private fun allowlistLines(): List<String> =
        allowlistFile.readLines().map { it.trim() }.filter { it.isNotEmpty() && !it.startsWith("#") }

    private fun readAllowlist(): Map<Key, Int> {
        val out = LinkedHashMap<Key, Int>()
        for (line in allowlistLines()) {
            val parts = line.split(Regex("\\s+"))
            require(parts.size == 3) { "allowlist line is not `<file> <pattern> <count>`: $line" }
            require(helperPatterns.any { it.id == parts[1] }) { "unknown pattern ${parts[1]} in: $line" }
            val count = requireNotNull(parts[2].toIntOrNull()?.takeIf { it > 0 }) { "count must be positive in: $line" }
            require(out.put(Key(parts[0], parts[1]), count) == null) { "duplicate allowlist entry: $line" }
        }
        return out
    }
}

/** A compiler-internal pattern that [helpers] in `support/` cover. */
internal class HelperPattern(val id: String, val directApi: String, val helpers: String, val regex: Regex)

internal val helperPatterns = listOf(
    HelperPattern("class-id-constructor", "the ClassId constructor", "classIdOf / nestedClassIdOf",
        Regex("""\bClassId\s*\(""")),
    HelperPattern("class-id-factory", "ClassId.topLevel / ClassId.fromString", "classIdOf / nestedClassIdOf",
        Regex("""\bClassId\s*\.\s*(?:topLevel|fromString)\s*\(""")),
    HelperPattern("class-type-constructor", "constructClassLikeType",
        "classType / nullableClassType / nullableStarProjectedType", Regex("""\bconstructClassLikeType\s*\(""")),
    HelperPattern("supertype-walk", "lookupSuperTypes", "inheritsFrom / inheritsFromAny / isOrInheritsFrom",
        Regex("""\blookupSuperTypes\s*\(""")),
    HelperPattern("smart-cast-unwrap", "FirSmartCastExpression.originalExpression", "unwrapSmartCasts",
        Regex("""\boriginalExpression\b""")),
    HelperPattern("safe-call-subject-unwrap", "FirCheckedSafeCallSubject.originalReceiverRef", "unwrapSmartCasts",
        Regex("""\boriginalReceiverRef\b""")),
    HelperPattern("not-null-unwrap", "FirCheckNotNullCall arguments",
        "unwrapSmartCasts(throughNotNull = true)",
        Regex("""\bFirCheckNotNullCall\b.*\barguments\b""")),
    HelperPattern("argument-unwrap", "FirWrappedArgumentExpression", "unwrapArgument",
        Regex("""\bFirWrappedArgumentExpression\b""")),
    HelperPattern("null-literal", "ConstantValueKind.Null", "isNullLiteral",
        Regex("""\bConstantValueKind\s*\.\s*Null\b""")),
    HelperPattern("trailing-lambda", "FirAnonymousFunctionExpression.isTrailingLambda", "trailingLambdaOf",
        Regex("""\bisTrailingLambda\b""")),
    HelperPattern("assignment-target", "FirDesugaredAssignmentValueReferenceExpression",
        "assignedAccess / isCompoundAssignment",
        Regex("""\bFirDesugaredAssignmentValueReferenceExpression\b""")),
    HelperPattern("light-children", "treeStructure.getChildren", "lightChildren / significantChildren",
        Regex("""\btreeStructure\s*\.\s*getChildren\s*\(""")),
    HelperPattern("light-text", "treeStructure.toString", "lightText",
        Regex("""\btreeStructure\s*\.\s*toString\s*\(""")),
    HelperPattern("light-source", "toKtLightSourceElement", "lightSourceOf",
        Regex("""\btoKtLightSourceElement\s*\(""")),
)

/** Matches of each [helperPatterns] entry in the code of [source], ignoring comments, strings, and imports. */
internal fun countPatterns(source: String): Map<String, Int> {
    val code = stripCommentsAndStrings(source).lineSequence().map { it.trim() }
        .filter { !it.startsWith("import ") && !it.startsWith("package ") }.toList()
    return helperPatterns.associate { p -> p.id to code.sumOf { p.regex.findAll(it).count() } }.filterValues { it > 0 }
}
