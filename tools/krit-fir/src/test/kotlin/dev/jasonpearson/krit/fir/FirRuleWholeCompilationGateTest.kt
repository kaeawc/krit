package dev.jasonpearson.krit.fir

import org.junit.jupiter.api.Test
import java.io.File
import kotlin.test.assertEquals
import kotlin.test.assertFalse
import kotlin.test.assertNotNull
import kotlin.test.assertNull
import kotlin.test.assertTrue

/**
 * Keeps [FirRule.wholeCompilationTier] honest, in both directions. A rule whose source reads facts of other
 * files cannot give a correct verdict on a slice of a module (the rest of the module is binary), and the incremental
 * runner (#865) will leave tiered rules to full compiles. Scans the checker sources on disk.
 *
 * What counts as a read of another file:
 *  - compilation-wide file state (`firProvider`, `getFirFile*`, package enumeration): kind COMPILATION_FILES;
 *  - another declaration's body or initializer: `resolvedInitializer`, `fir.body`, `fir.initializer`,
 *    `fir.defaultValue`, a symbol's `delegate`: kind OTHER_DECLARATION_BODIES;
 *  - `source` of a symbol-typed receiver and `getContainingFile*`, which exist only for a declaration compiled from
 *    source: OTHER_DECLARATION_BODIES.
 *
 * Two-way: a rule with such a read and no tier fails, and a rule with a tier but no read fails. A read that is
 * proven not to cross files is listed in [allowedReads] with its proof, and then does not count.
 * The split differential tests check the tiering against real slice compiles.
 * Known gap this gate cannot see: SOURCE-retention annotations are absent from class files (findings section 8.2).
 */
class FirRuleWholeCompilationGateTest {
    private val checkersDir = File("src/main/kotlin/dev/jasonpearson/krit/fir/checkers")

    private val ruleIdDecl = Regex("""\bval\s+ruleId\s*(?::\s*String\s*)?=\s*"([^"]+)"""")

    private val compilationWide = Regex("""\b(firProvider|getFirFile\w*|getPackageNames|symbolNamesProvider)\b""")

    // Per-symbol `symbolProvider` lookups are fine (class and signature metadata survive in a snapshot); these are not.
    private val bodyReads = listOf(
        Regex("""\bresolvedInitializer\b"""),
        Regex("""\bfir\??\.(body|initializer|defaultValue)\b"""),
        Regex("""\b(symbol|\w*Symbol|property|function|variable|getter|accessor|resolved|klass|owner)\??\.(source|delegate)\b"""),
        Regex("""\bgetContainingFile\w*\b"""),
        // A FIR element taken from a symbol and kept in a variable (`val property = symbol.fir`) hides the later
        // `.initializer`/`.getter` reads, so the symbol-to-FIR step itself counts.
        Regex("""\b\w+\??\.fir\b"""),
    )

    /** Local: the symbol is declared in the file under check. Const: the value is kept in class files. */
    private enum class Why { LOCAL, CONST, RECORDED }

    /**
     * A source line that matches a read pattern but is excluded from the gate. [line] is matched trimmed and exact
     * against the comment- and string-stripped source; [occurrence] picks the nth identical line of the rule's file
     * (0 = first), so a second identical line is never allowed silently.
     */
    private data class AllowedRead(
        val ruleId: String,
        val line: String,
        val why: Why,
        val note: String,
        val occurrence: Int = 0,
    )

    private val allowedReads = listOf(
        AllowedRead("Wakelock", "return assignment?.rValue ?: symbol.resolvedInitializer", Why.LOCAL,
            "valueAt is reached only for FirPropertySymbol.isLocal (the `symbol.isLocal` branch at its call site)"),
        AllowedRead("Wakelock", "val declared = property.source ?: return false", Why.LOCAL,
            "declaredFromNewWakeLock returns early unless property.isLocal"),
        AllowedRead("Wakelock", "val function = candidate.function.source ?: return false", Why.LOCAL,
            "candidate.function is the function declaration under check"),
        AllowedRead("Wakelock", "var initializer = property.resolvedInitializer ?: return false", Why.LOCAL,
            "declaredFromNewWakeLock returns early unless property.isLocal"),
        AllowedRead("BufferedReadWithoutBuffer", "val initializer = symbol.resolvedInitializer", Why.LOCAL,
            "sourceOfLocal returns early unless symbol.isLocal"),
        AllowedRead("BufferedReadWithoutBuffer", "val declaration = symbol.source?.let { Span(it.startOffset, it.endOffset) }", Why.LOCAL,
            "reaching() is called only from sourceOfLocal, with a local symbol"),
        AllowedRead("JdbcPreparedStatementNotClosed", "if (getter != null && getter.source?.kind is KtRealSourceElementKind) {", Why.LOCAL,
            "the getter of the declaration under check"),
        AllowedRead("JdbcPreparedStatementNotClosed", "val source = property.source ?: return false", Why.LOCAL,
            "a FirProperty element met while walking the declaration under check"),
        AllowedRead("SynchronizedOnBoxedPrimitive", "val source = property.source ?: return false", Why.LOCAL,
            "a FirProperty element met while walking the class under check"),
        AllowedRead("UnicodeNormalizationMissing", "val source = function.source ?: return false", Why.LOCAL,
            "the function containing the call under check (context.containingDeclarations)"),
        AllowedRead("BindsReturnTypeMatchesParam", "val default = parameter.fir.defaultValue", Why.CONST,
            "an annotation constructor's parameter default; annotation defaults are kept in class files"),
        AllowedRead("BindsReturnTypeMatchesParam", "val result = FirExpressionEvaluator.evaluateParameterDefaultValue(parameter.fir, session)", Why.CONST,
            "evaluates the same annotation constructor default; kept in class files"),
        AllowedRead("UseEmptyCounterpart", "val imports = context.containingFileSymbol?.fir?.imports.orEmpty()", Why.LOCAL,
            "containingFileSymbol is the file under check"),
        AllowedRead("PrngFromSystemTime", "val imports = context.containingFileSymbol?.fir?.imports ?: return false", Why.LOCAL,
            "containingFileSymbol is the file under check"),
        AllowedRead("SynchronizedOnBoxedPrimitive", "owner.fir.accept(object : FirVisitorVoid() {", Why.LOCAL,
            "owner is the enclosing class of the code under check (context.containingDeclarations)"),
        AllowedRead("BufferedReadWithoutBuffer", "val root = rootSymbol.fir", Why.LOCAL,
            "rootSymbol is the outermost non-class declaration enclosing the code under check (context.containingDeclarations)"),
        AllowedRead("ReassignedLocals", "root.fir.accept(object : FirVisitorVoid() {", Why.LOCAL,
            "root is the outermost non-class declaration enclosing the code under check (context.containingDeclarations); helper file, named by file name"),
        // SerialVersionUIDInSerializableClass walks the files the compile holds as source and adds the recorded
        // class-name facts of every other module file (FirRuleCompileContext.externalClassNameFacts), so a slice and a
        // full compile build the same set. FirIncrementalSplitDifferentialTest and the planner tests pin that.
        AllowedRead("SerialVersionUIDInSerializableClass", "val provider = session.firProvider", Why.RECORDED,
            "the compile's own source files; the other module files arrive as externalClassNameFacts"),
        AllowedRead("SerialVersionUIDInSerializableClass", "for (pkg in provider.symbolProvider.symbolNamesProvider.getPackageNames().orEmpty()) {", Why.RECORDED,
            "enumerates the packages of the compile's own source files; the other files arrive as externalClassNameFacts"),
        AllowedRead("SerialVersionUIDInSerializableClass", "for (file in provider.getFirFilesByPackage(FqName(pkg))) facts += classNameFacts(file)", Why.RECORDED,
            "the compile's own source files; the other files arrive as externalClassNameFacts"),
        AllowedRead("WorldReadableFiles", "val scope = klass.fir.scopeProvider.getStaticScope(klass.fir, context.session, context.scopeSession)", Why.CONST,
            "reads the static scope (member signatures and supertypes) of an imported class, which class files keep; no body or initializer is read"),
    )

    private class Hit(val file: File, val line: String, val compilationWide: Boolean)

    @Test fun flagMatchesSourceUsage() {
        assertTrue(checkersDir.isDirectory, "run from tools/krit-fir; missing ${checkersDir.absolutePath}")
        val tiers = FirRuleDiscovery.rules.associate { it.ruleId to it.wholeCompilationTier }
        val problems = mutableListOf<String>()
        val hitsByRule = HashMap<String, List<Hit>>()
        for (file in checkersDir.walkTopDown().filter { it.isFile && it.extension == "kt" }.sortedBy { it.path }) {
            val source = stripCommentsAndStrings(file.readText())
            val hits = readLines(source).map { Hit(file, it.first, it.second) }
            val ids = ruleIdDecl.findAll(file.readText()).map { it.groupValues[1] }.toList()
            if (ids.isEmpty()) {
                // A helper file with no rule id: allow-list entries name it by file name.
                hitsByRule[file.nameWithoutExtension] = hits
                val counted = countedHits(file.nameWithoutExtension, hits)
                if (counted.isNotEmpty()) {
                    problems += "${file.path} reads other-file state (${counted.joinToString { it.line.take(60) }}) but " +
                        "declares no rule id. Move the read into the rule's own file (and set " +
                        "wholeCompilationTier there), stop reading other declarations, or, if the read is proven " +
                        "local or const, add an allowedReads entry named by the file name."
                }
                continue
            }
            for (id in ids) hitsByRule[id] = hits
        }
        for ((id, hits) in hitsByRule.toSortedMap()) {
            if (id !in tiers) continue
            val counted = countedHits(id, hits)
            val tier = tiers[id]
            val wide = counted.any { it.compilationWide }
            if (counted.isNotEmpty() && tier == null) {
                val fn = if (wide) "compilationFiles" else "otherDeclarationBodies"
                problems += "$id (${counted.first().file.path}) reads facts of other declarations or files: " +
                    counted.joinToString("; ") { "`${it.line.take(90)}`" } + ". A slice compile reads a declaration " +
                    "from a binary snapshot without its body or initializer, so the verdict changes. Add " +
                    "`override val wholeCompilationTier = $fn(\"<the fact it reads>\")` to the rule. If the read is " +
                    "provably local (a local variable) or const, add it to allowedReads in this test with the proof."
            }
            if (counted.isEmpty() && tier != null) {
                problems += "$id sets wholeCompilationTier but no longer reads other declarations' bodies or " +
                    "initializers or the compilation's files. Remove the override so incremental runs check it, " +
                    "or, if the read is still there but provably local, list it in allowedReads."
            }
            if (tier != null && wide && tier.kind != WholeCompilationKind.COMPILATION_FILES) {
                problems += "$id iterates the compilation's files; use compilationFiles(...) rather than ${tier.kind}."
            }
            if (tier != null && !wide && tier.kind == WholeCompilationKind.COMPILATION_FILES && counted.isNotEmpty()) {
                problems += "$id uses compilationFiles(...) but no longer iterates the compilation's files; use " +
                    "otherDeclarationBodies(...) for its remaining reads."
            }
        }
        for (a in allowedReads) {
            val hits = hitsByRule[a.ruleId].orEmpty()
            if (hits.count { it.line == a.line } <= a.occurrence) {
                problems += "allowedReads entry for ${a.ruleId} (occurrence ${a.occurrence}) matches no read line: " +
                    "`${a.line}`. The code moved or changed; update or delete the entry (and re-check that the " +
                    "read is still ${a.why.name.lowercase()}). Current read lines of ${a.ruleId}:\n    " +
                    hits.joinToString("\n    ") { it.line }
            }
            if (tiers[a.ruleId] != null) {
                problems += "allowedReads lists ${a.ruleId}, which is tiered. Delete the entry; tiered rules need no allowance."
            }
        }
        assertTrue(problems.isEmpty(), problems.joinToString("\n"))
    }

    /** The hits not covered by an allow-list entry; the nth identical line is covered only by occurrence n. */
    private fun countedHits(ruleId: String, hits: List<Hit>): List<Hit> {
        val seen = HashMap<String, Int>()
        return hits.filter { h ->
            val n = seen.merge(h.line, 1, Int::plus)!! - 1
            allowedReads.none { it.ruleId == ruleId && it.line == h.line && it.occurrence == n }
        }
    }

    @Test fun everyTieredRuleStatesAReason() {
        val tiers = FirRuleDiscovery.wholeCompilationTiers()
        assertEquals(FirRuleDiscovery.wholeCompilationRuleIds(), tiers.keys.toList())
        for ((id, tier) in tiers) {
            assertTrue(tier.reason.length > 20, "$id needs a real reason: '${tier.reason}'")
        }
    }

    @Test fun theRulesTheDifferentialsFoundAreTiered() {
        // Findings doc section 8.1 (single-file split) plus the cross-file probes of the review.
        val found = listOf(
            "CollectionsSynchronizedListIteration", "HardcodedSecretKey", "JdbcStatementExecute",
            "LocaleGetDefaultForFormatting", "LongLogTag",
            "SharedFlowWithoutReplay", "ShortAlarm", "StaticIv", "WeakKeySize", "WorldWriteableFiles",
            "SecureRandom", "ShowToast", "GetSignatures", "TooGenericExceptionThrown", "GlobalScopeLaunchInViewModel",
        )
        val tiered = FirRuleDiscovery.wholeCompilationRuleIds()
        assertTrue(tiered.containsAll(found), "missing: ${found - tiered.toSet()}")
        // Served on slices through recorded class-name facts, so it is not tiered any more.
        assertFalse("SerialVersionUIDInSerializableClass" in tiered)
    }

    @Test fun ruleWithoutOverrideIsNotTiered() {
        val plain = object : FirRule { override val ruleId = "Plain" }
        assertNull(plain.wholeCompilationTier)
        assertFalse(plain.requiresWholeCompilation)
        assertFalse(FirRuleDiscovery.rules.first { it.ruleId == "InjectDispatcher" }.requiresWholeCompilation)
        assertFalse(FirRuleDiscovery.rules.first { it.ruleId == "SerialVersionUIDInSerializableClass" }.requiresWholeCompilation)
        val tiered = FirRuleDiscovery.rules.first { it.ruleId == "LongLogTag" }
        assertTrue(tiered.requiresWholeCompilation)
        assertNotNull(tiered.wholeCompilationTier)
    }

    @Test fun scannerIgnoresCommentsAndStringsAndFlagsRealReads() {
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
            val i = "resolvedInitializer in a string"
            val j = property.source
            val k = symbol.delegate
            val l = parameter.fir.defaultValue
            val m = symbol.fir
            package a.b.fir
            import a.b.fir.X
        """.trimIndent()
        assertEquals(
            listOf(
                "val a = session.firProvider", "val b = symbol.resolvedInitializer", "val c = getter.fir.body",
                "val d = klass.fir?.initializer", "val e = functionSymbol.source",
                "val g = containingFileSymbol?.fir?.imports", "val j = property.source",
                "val k = symbol.delegate", "val l = parameter.fir.defaultValue", "val m = symbol.fir",
            ),
            readLines(stripCommentsAndStrings(src)).map { it.first },
        )
    }

    @Test fun stripperRespectsLexicalStructure() {
        fun strip(s: String) = stripCommentsAndStrings(s)
        // Nested block comments end at the matching close.
        assertEquals("a\nb", strip("a/* x /* y */ still comment */\nb"))
        // Comment markers inside strings are not comments; string contents are blanked.
        assertEquals("val u = \"\"\nval v = 1 ", strip("val u = \"http://x /* y\"\nval v = 1 // tail"))
        // Raw strings may hold quotes, comment markers and newlines.
        assertEquals("val r = \"\"\"\n\"\"\"\nnext", strip("val r = \"\"\"a \"b\" // c\n/* d\"\"\"\nnext"))
        // Escapes and char literals (a char literal's content is blanked to x).
        assertEquals("val q = \"\" + 'x' ", strip("val q = \"a\\\"b\" + 'x' // c"))
        assertEquals("val d = 'x'\nval e = 2 ", strip("val d = '\"'\nval e = 2 /* c */"))
        // Code after an unterminated-looking apostrophe in a comment still scans.
        assertEquals("x \ny", strip("x // don't\ny"))
    }

    /** (line, isCompilationWide) of every read line in comment- and string-stripped [source]. */
    private fun readLines(source: String): List<Pair<String, Boolean>> =
        source.lineSequence().map { it.trim() }.filter { it.isNotEmpty() && !it.startsWith("import ") && !it.startsWith("package ") }
            .mapNotNull { line ->
                val wide = compilationWide.containsMatchIn(line)
                if (wide || bodyReads.any { it.containsMatchIn(line) }) line to wide else null
            }.toList()

    /**
     * Removes comments (line, and block comments which nest) and blanks the contents of string literals (plain and
     * raw) and char literals, keeping the quotes and every newline so line structure survives.
     */
    private fun stripCommentsAndStrings(src: String): String {
        val out = StringBuilder()
        var i = 0
        val n = src.length
        while (i < n) {
            val c = src[i]
            when {
                src.startsWith("//", i) -> while (i < n && src[i] != '\n') i++
                src.startsWith("/*", i) -> {
                    var depth = 1
                    i += 2
                    while (i < n && depth > 0) {
                        when {
                            src.startsWith("/*", i) -> { depth++; i += 2 }
                            src.startsWith("*/", i) -> { depth--; i += 2 }
                            else -> { if (src[i] == '\n') out.append('\n'); i++ }
                        }
                    }
                }
                src.startsWith("\"\"\"", i) -> {
                    out.append("\"\"\"")
                    i += 3
                    while (i < n && !src.startsWith("\"\"\"", i)) { if (src[i] == '\n') out.append('\n'); i++ }
                    // A raw string may end with extra quotes ("""" is one quote of content plus the close).
                    while (i < n && src.startsWith("\"\"\"\"", i)) i++
                    if (i < n) { out.append("\"\"\""); i += 3 }
                }
                c == '"' -> {
                    out.append('"')
                    i++
                    while (i < n && src[i] != '"') { if (src[i] == '\\') i++; i++ }
                    out.append('"')
                    i++
                }
                c == '\'' -> {
                    // A char literal is 'x' or '\x...'; an apostrophe that is not one is left alone.
                    val close = if (i + 1 < n && src[i + 1] == '\\') src.indexOf('\'', i + 3) else if (i + 2 < n && src[i + 2] == '\'') i + 2 else -1
                    if (close in (i + 1)..(i + 12)) { out.append('\'').append('x').append('\''); i = close + 1 } else { out.append(c); i++ }
                }
                else -> { out.append(c); i++ }
            }
        }
        return out.toString()
    }
}
