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
    private enum class Why { LOCAL, CONST }

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
        AllowedRead("WorldReadableFiles", "val scope = klass.fir.scopeProvider.getStaticScope(klass.fir, context.session, context.scopeSession)", Why.CONST,
            "reads the static scope (member signatures and supertypes) of an imported class, which class files keep; no body or initializer is read"),
        // Narrowed rules (#873): a rule reads another declaration only through SameFileReads, or at a spot proven below.
        AllowedRead("SameFileReads", "(origin == FirDeclarationOrigin.Source && session.firProvider.getContainingFile(this)?.symbol == file)", Why.LOCAL,
            "the test that decides whether a symbol lies in the file under check: it reads which file contains the symbol, no body or initializer"),
        AllowedRead("SameFileReads", "if (isDeclaredInFile(file, session)) resolvedInitializer else null", Why.LOCAL,
            "guarded by isDeclaredInFile: only a declaration of the file under check"),
        AllowedRead("SameFileReads", "if (isDeclaredInFile(file, session)) return resolvedInitializer", Why.LOCAL,
            "guarded by isDeclaredInFile: only a declaration of the file under check"),
        AllowedRead("SameFileReads", "return evaluated ?: resolvedInitializer", Why.CONST,
            "reached only for a const outside the file under check; the evaluated value is what a class file holds"),
        AllowedRead("SameFileReads", "if (isDeclaredInFile(file, session)) fir.body else null", Why.LOCAL,
            "guarded by isDeclaredInFile: only a declaration of the file under check"),
        AllowedRead("GetSignatures", "val propertySource = property.source ?: return true", Why.LOCAL,
            "accessorInsideDeclaration compares a property met on the call's enclosing path (context.containingElements) with its accessor"),
        AllowedRead("GetSignatures", "val accessorSource = accessor.source?.takeIf { it.kind == KtRealSourceElementKind } ?: return true", Why.LOCAL,
            "the accessor of that same enclosing property"),
        AllowedRead("GetSignatures", "is FirFieldSymbol -> if (symbol.fir.isVal) symbol.fir.initializer else null", Why.CONST,
            "a Java field: its constant value is kept in class files (and a Java source file is never in the snapshot)"),
        AllowedRead("GetSignatures", "is FirFieldSymbol -> original.fir.initializer", Why.CONST,
            "a Java field: its constant value is kept in class files"),
        AllowedRead("GetSignatures", "val rootFir = root.fir", Why.LOCAL,
            "root is the outermost non-class declaration enclosing the call (context.containingDeclarations)"),
        AllowedRead("GetSignatures", "?: (listOfNotNull(symbol.resolvedInitializer) + assignedValues(symbol, rootFir))", Why.LOCAL,
            "this branch is reached only for a local var (localVar), declared in the enclosing declaration"),
        AllowedRead("LongLogTag", "val file = context.containingFileSymbol?.fir ?: return null", Why.LOCAL,
            "containingFileSymbol is the file under check"),
        AllowedRead("LongLogTag", "root.fir.accept(scan)", Why.LOCAL,
            "root is the outermost non-class declaration enclosing the call (context.containingDeclarations)"),
        AllowedRead("LongLogTag", "if (scan.assignments.isEmpty()) return variable.resolvedInitializer", Why.LOCAL,
            "localVarValue is reached only for a local var (symbol.isLocal && !symbol.isVal)"),
        AllowedRead("LongLogTag", "return if (deferred >= 0) null else variable.resolvedInitializer", Why.LOCAL,
            "localVarValue is reached only for a local var (symbol.isLocal && !symbol.isVal)"),
        AllowedRead("SharedFlowWithoutReplay", "return symbol.resolvedInitializer", Why.LOCAL,
            "localValInitializer returns early unless symbol.isLocal"),
        AllowedRead("ShowToast", "val file = context.containingFileSymbol?.fir ?: return", Why.LOCAL,
            "containingFileSymbol is the file under check"),
        AllowedRead("WeakKeySize", "val writes = Writes().also { symbol.fir.accept(it) }", Why.LOCAL,
            "reached only when symbol.isDeclaredInCheckedFile() (algorithmsReaching)"),
        AllowedRead("WeakKeySize", "val property = symbol.fir", Why.LOCAL,
            "isFinalInitializedVal is called only after symbol.isDeclaredInCheckedFile() (algorithmsReaching)"),
        AllowedRead("WeakKeySize", "return symbol.isVal && property.initializer != null && property.delegate == null &&", Why.LOCAL,
            "reads the FirProperty taken on the previous line, under the same guard"),
        AllowedRead("WeakKeySize", "val writes = Writes().also { region.fir.accept(it) }", Why.LOCAL,
            "region is a declaration enclosing the call (context.containingDeclarations)"),
        AllowedRead("WeakKeySize", "val writes = Writes().also { function.fir.accept(it) }", Why.LOCAL,
            "function is a named function enclosing the call (context.containingDeclarations)"),
        AllowedRead("WeakKeySize", "applyLambda(symbol.fir.initializer)?.let { return lambdaKey(it, depth + 1) }", Why.LOCAL,
            "guarded by symbol.isLocal && symbol.isVal"),
        AllowedRead("WeakKeySize", "return constantValue(symbol.resolvedInitializer, depth + 1)", Why.CONST,
            "constantString returns early unless symbol.isConst; a const value is kept in class files"),
        AllowedRead("WorldWriteableFiles", "val source = resolved.source ?: continue", Why.LOCAL,
            "an import of the file under check (FirFile.imports)"),
        AllowedRead("WorldWriteableFiles", "val default = if (symbol.isDeclaredInCheckedFile()) symbol.fir.defaultValue else null", Why.LOCAL,
            "guarded by isDeclaredInCheckedFile"),
        AllowedRead("WorldWriteableFiles", "val literal = symbol.resolvedInitializer as? FirLiteralExpression ?: return Value.Unknown", Why.CONST,
            "a Java field: its constant value is kept in class files"),
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

    @Test fun onlyTheRulesThatCannotBeNarrowedAreTiered() {
        // The 15 rules the differentials found (findings doc section 8.1 and the cross-file probes) read other
        // files' bodies; #873 narrowed them to the file under check plus what class files keep, so they run on
        // slices. SerialVersionUIDInSerializableClass walks every source file of the module and stays tiered
        // until it gets the module's class index from the incremental state.
        val narrowed = listOf(
            "CollectionsSynchronizedListIteration", "HardcodedSecretKey", "JdbcStatementExecute",
            "LocaleGetDefaultForFormatting", "LongLogTag", "SharedFlowWithoutReplay", "ShortAlarm", "StaticIv",
            "WeakKeySize", "WorldWriteableFiles", "SecureRandom", "ShowToast", "GetSignatures",
            "TooGenericExceptionThrown", "GlobalScopeLaunchInViewModel",
        )
        val tiered = FirRuleDiscovery.wholeCompilationRuleIds()
        val ids = FirRuleDiscovery.rules.map { it.ruleId }
        assertTrue(ids.containsAll(narrowed), "rule missing: ${narrowed - ids.toSet()}")
        assertTrue(tiered.none { it in narrowed }, "narrowed rules must not be tiered: ${tiered.filter { it in narrowed }}")
        assertEquals(listOf("SerialVersionUIDInSerializableClass"), tiered)
        assertEquals(WholeCompilationKind.COMPILATION_FILES, FirRuleDiscovery.wholeCompilationTiers()["SerialVersionUIDInSerializableClass"]?.kind)
    }

    @Test fun ruleWithoutOverrideIsNotTiered() {
        val plain = object : FirRule { override val ruleId = "Plain" }
        assertNull(plain.wholeCompilationTier)
        assertFalse(plain.requiresWholeCompilation)
        assertFalse(FirRuleDiscovery.rules.first { it.ruleId == "InjectDispatcher" }.requiresWholeCompilation)
        val tiered = FirRuleDiscovery.rules.first { it.ruleId == "SerialVersionUIDInSerializableClass" }
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
