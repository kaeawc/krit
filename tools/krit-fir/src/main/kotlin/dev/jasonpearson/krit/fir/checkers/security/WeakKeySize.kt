package dev.jasonpearson.krit.fir.checkers.security

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.impl.FirDefaultPropertyGetter
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirComponentCall
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirValueParameterSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirVariableSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.SpecialNames

/**
 * Flags `javax.crypto.KeyGenerator.init(size, ...)` and
 * `java.security.KeyPairGenerator.initialize(size, ...)` whose first argument
 * is an integer literal below the per-algorithm minimum (RSA and DSA 2048, EC
 * and ECDSA 224, AES 128, any HmacSHA* 256), where the generator is a
 * variable named by a bare identifier whose value comes from
 * `KeyGenerator.getInstance("<alg>")` or `KeyPairGenerator.getInstance("<alg>")`
 * with a plain string literal algorithm.
 *
 * Mirrors the Go rule's evidence on top of FIR resolution:
 * - the call must resolve to KeyGenerator.init or KeyPairGenerator.initialize
 *   (any overload), with a bare variable as its receiver (`gen.init(64)`,
 *   `gen?.init(64)`, a smart-cast `gen`);
 * - the size is the first argument's source text, read as Go reads it: a
 *   decimal integer with an optional sign, `_` separators, and an `L`/`l`
 *   suffix, so a hex literal or a constant never counts;
 * - the algorithm comes from a write to that variable (its initializer or an
 *   assignment, a destructuring entry taking the whole declaration's value)
 *   that precedes the call in the source: the first getInstance call on
 *   KeyGenerator or KeyPairGenerator with a string literal argument inside the
 *   written value whose nearest enclosing write is that write (a named
 *   function in between stops the attribution, as Go stops at a
 *   function_declaration);
 * - the algorithm text is normalized as Go does: surrounding whitespace
 *   trimmed with Go's unicode.IsSpace set, `-` removed, uppercased; the
 *   literal's escape entries keep their source text.
 * The writes searched are those of the declaration that holds every write of
 * a local variable (the outermost enclosing function, property initializer, or
 * init block), and, for a member or top-level property, those of the nearest
 * enclosing named function (else the nearest enclosing constructor, init
 * block, accessor, or property initializer). A final member or top-level `val`
 * with an initializer and a default getter takes the algorithm of its
 * initializer.
 *
 * Deliberate differences from Go, pinned by goldens. Go matches the variable
 * by name and the getInstance receiver by spelling; FIR resolves both:
 * - Recall (Go misses these true positives, WeakKeySizeGoMisses and
 *   WeakKeySizeDeclaredName): the getInstance receiver spelled through an
 *   import alias, a typealias, or parentheses, or in a file that also declares
 *   an unrelated class named KeyGenerator or KeyPairGenerator; a write outside
 *   Go's enclosing function_declaration (a local variable used in a local
 *   function or local class method, a variable in an init block or property
 *   initializer); a final member or top-level `val` initialized with
 *   getInstance; a `when` subject variable; a later conditional write whose
 *   algorithm makes the size weak; a variable named like one declared earlier
 *   in the function; an annotated, labeled, or comment-led argument; a
 *   parenthesized or backticked receiver.
 * - Precision (Go reports these, but the generator does not hold the
 *   algorithm Go reads, WeakKeySizePrecision): a receiver that is a lambda
 *   parameter or another variable sharing a getInstance variable's name; a
 *   variable reassigned on every path before the call; a receiver that is not
 *   a KeyGenerator or KeyPairGenerator; a getInstance call on a local that
 *   shadows the KeyGenerator import.
 */
internal object WeakKeySize : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "WeakKeySize"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(WeakKeySize)
    }

    private const val MESSAGE =
        "Crypto key generator initialized with a weak literal key size. Use a size that meets the algorithm's current minimum strength."

    private val keyGenerator = ClassId(FqName("javax.crypto"), Name.identifier("KeyGenerator"))
    private val keyPairGenerator = ClassId(FqName("java.security"), Name.identifier("KeyPairGenerator"))
    private val initIds = setOf(
        CallableId(keyGenerator, Name.identifier("init")),
        CallableId(keyPairGenerator, Name.identifier("initialize")),
    )
    private val getInstanceIds = setOf(
        CallableId(keyGenerator, Name.identifier("getInstance")),
        CallableId(keyPairGenerator, Name.identifier("getInstance")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.callableId !in initIds) return
        val source = expression.source ?: return
        val receiver = variableReceiver(expression.explicitReceiver) ?: return
        val size = firstArgumentSize(source) ?: return
        val callStart = source.startOffset
        val algorithms = algorithmsReaching(receiver, callStart) ?: return
        val weak = algorithms.any { algorithm ->
            val threshold = threshold(algorithm)
            threshold != null && size < threshold
        }
        if (weak) report(source, MESSAGE)
    }

    // The variable a bare-identifier receiver names: `gen`, `gen?.`, or a
    // smart-cast `gen`, a property or a Java field. Go takes only a receiver
    // without a dot.
    private fun variableReceiver(receiver: FirExpression?): FirVariableSymbol<*>? {
        var current = receiver
        while (true) {
            current = when (current) {
                is FirCheckedSafeCallSubject -> current.originalReceiverRef.value
                is FirSmartCastExpression -> current.originalExpression
                else -> break
            }
        }
        val access = current as? FirPropertyAccessExpression ?: return null
        if (access.explicitReceiver != null) return null
        return access.calleeReference.toResolvedCallableSymbol() as? FirVariableSymbol<*>
    }

    // The algorithms [symbol] may hold at [callStart], or null when unknown.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun algorithmsReaching(symbol: FirVariableSymbol<*>, callStart: Int): List<String>? {
        val declarations = context.containingDeclarations
        val local = isLocal(symbol)
        if (symbol is FirPropertySymbol && !local) {
            val property = symbol.fir
            val initializer = property.initializer
            if (symbol.isVal && initializer != null && property.delegate == null &&
                (property.getter == null || property.getter is FirDefaultPropertyGetter) &&
                symbol.resolvedStatus.modality == Modality.FINAL
            ) {
                val writes = Writes(Int.MAX_VALUE).also { property.accept(it) }
                return listOfNotNull(writes.of(symbol, local).firstOrNull()?.algorithm(Int.MAX_VALUE))
            }
        }
        val region = if (local) {
            declarations.firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol }
        } else {
            declarations.lastOrNull { it is FirNamedFunctionSymbol }
                ?: declarations.lastOrNull {
                    it !is FirClassLikeSymbol<*> && it !is FirFileSymbol && it !is FirAnonymousFunctionSymbol
                }
        } ?: return null
        val writes = Writes(callStart).also { region.fir.accept(it) }
        val before = writes.of(symbol, local).filter { it.start < callStart }.sortedBy { it.start }
        // A write that runs on every path to the call kills the ones before it.
        val last = before.indexOfLast { it.dominates(callStart) }
        val reaching = if (last >= 0) before.subList(last, before.size) else before
        return reaching.mapNotNull { it.algorithm(callStart) }
    }

    // A local variable or a parameter: its writes all lie in its declaring
    // function, and its callable id does not identify it.
    private fun isLocal(symbol: FirVariableSymbol<*>): Boolean =
        (symbol is FirPropertySymbol && symbol.isLocal) || symbol is FirValueParameterSymbol

    private class Instance(val start: Int, val algorithm: String)

    private class Write(val target: FirBasedSymbol<*>, val start: Int, private val block: KtSourceElement?) {
        val instances = ArrayList<Instance>()
        var aliasOf: Write? = null

        // True when the write is a statement of a block that contains [offset].
        fun dominates(offset: Int): Boolean = block != null && block.startOffset <= offset && offset < block.endOffset

        // The first getInstance algorithm attributed to this write that
        // starts before [before], as Go takes the first in source order.
        fun algorithm(before: Int): String? =
            (aliasOf ?: this).instances.filter { it.start < before }.minByOrNull { it.start }?.algorithm
    }

    // Collects every write in a declaration, attributing each qualifying
    // getInstance call to its nearest enclosing write.
    private class Writes(private val callStart: Int) : FirVisitorVoid() {
        private val all = ArrayList<Write>()
        private val blockOf = HashMap<FirElement, KtSourceElement?>()
        private val destructs = HashMap<FirBasedSymbol<*>, Write>()
        private var current: Write? = null

        // The writes to [symbol]; a member or top-level property or a field
        // also matches through another symbol for the same declaration (a
        // fake override, a synthetic Java property).
        fun of(symbol: FirVariableSymbol<*>, local: Boolean): List<Write> = all.filter {
            it.target == symbol ||
                (!local && it.target is FirVariableSymbol<*> && !isLocal(it.target) && it.target.callableId == symbol.callableId)
        }

        override fun visitElement(element: FirElement) {
            when (element) {
                is FirBlock -> {
                    for (statement in element.statements) blockOf[statement] = element.source
                    element.acceptChildren(this)
                }
                is FirNamedFunction -> within(null) { element.acceptChildren(this) }
                is FirProperty -> {
                    val initializer = element.initializer
                    if (initializer == null) {
                        element.acceptChildren(this)
                        return
                    }
                    val write = Write(element.symbol, element.source?.startOffset ?: Int.MAX_VALUE, blockOf[element])
                    all += write
                    if (element.name == SpecialNames.DESTRUCT) destructs[element.symbol] = write
                    if (initializer is FirComponentCall) {
                        val subject = (initializer.explicitReceiver as? FirQualifiedAccessExpression)
                            ?.calleeReference?.toResolvedCallableSymbol()
                        write.aliasOf = subject?.let { destructs[it] }
                    }
                    within(write) { element.acceptChildren(this) }
                }
                is FirVariableAssignment -> {
                    val lValue = element.lValue
                    val target = (if (lValue is FirDesugaredAssignmentValueReferenceExpression) lValue.expressionRef.value else lValue)
                        as? FirQualifiedAccessExpression
                    val symbol = target?.calleeReference?.toResolvedCallableSymbol()
                    if (symbol == null) {
                        element.acceptChildren(this)
                        return
                    }
                    val write = Write(symbol, element.source?.startOffset ?: Int.MAX_VALUE, blockOf[element])
                    all += write
                    lValue.accept(this)
                    within(write) { element.rValue.accept(this) }
                }
                is FirFunctionCall -> {
                    val write = current
                    val start = element.source?.startOffset
                    if (write != null && start != null && start < callStart) {
                        val callee = element.calleeReference.toResolvedCallableSymbol()
                        if (callee != null && callee.callableId in getInstanceIds) {
                            firstArgumentString(element.source)?.let { write.instances += Instance(start, it) }
                        }
                    }
                    element.acceptChildren(this)
                }
                else -> element.acceptChildren(this)
            }
        }

        private inline fun within(write: Write?, block: () -> Unit) {
            val saved = current
            current = write
            try {
                block()
            } finally {
                current = saved
            }
        }
    }

    // Syntax wrappers that leave the argument's value unchanged, and the
    // non-expression parts inside them.
    private val expressionWrappers = setOf(
        KtNodeTypes.PARENTHESIZED,
        KtNodeTypes.ANNOTATED_EXPRESSION,
        KtNodeTypes.LABELED_EXPRESSION,
    )
    private val wrapperParts = setOf(
        KtTokens.LPAR,
        KtTokens.RPAR,
        KtNodeTypes.ANNOTATION_ENTRY,
        KtNodeTypes.ANNOTATION,
        KtNodeTypes.LABEL_QUALIFIER,
    )

    // The first argument's expression node of the call [source] spans, with
    // parentheses, annotations, and labels unwrapped.
    private fun firstArgument(source: KtSourceElement): LighterASTNode? {
        val root = source.lighterASTNode
        val call = when (root.tokenType) {
            KtNodeTypes.CALL_EXPRESSION -> root
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION ->
                significantChildren(source, root).lastOrNull()?.takeIf { it.tokenType == KtNodeTypes.CALL_EXPRESSION }
            else -> null
        } ?: return null
        val arguments = significantChildren(source, call)
            .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST } ?: return null
        val argument = significantChildren(source, arguments)
            .firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT } ?: return null
        // A named (`name = x`) or spread (`*x`) argument has more than one part.
        var value = significantChildren(source, argument).singleOrNull() ?: return null
        while (value.tokenType in expressionWrappers) {
            value = significantChildren(source, value)
                .filter { it.tokenType !in wrapperParts }
                .singleOrNull() ?: return null
        }
        return value
    }

    // Go's reading of the size: the argument text, trimmed, without an `L`
    // then an `l` suffix and without `_`, parsed as a signed decimal integer.
    private fun firstArgumentSize(source: KtSourceElement): Long? {
        val node = firstArgument(source) ?: return null
        var text = trimGoSpace(source.treeStructure.toString(node).toString())
        text = text.removeSuffix("L").removeSuffix("l").replace("_", "")
        return parseGoInt(text)
    }

    // strconv.Atoi: an optional sign, then ASCII digits, within 64 bits.
    private fun parseGoInt(text: String): Long? {
        val digits = if (text.startsWith("+") || text.startsWith("-")) text.substring(1) else text
        if (digits.isEmpty() || digits.any { it !in '0'..'9' }) return null
        return text.toLongOrNull()
    }

    // The raw text of the first argument when it is a string template without
    // `$` entries; escape entries keep their source text, as Go reads them.
    private fun firstArgumentString(source: KtSourceElement?): String? {
        source ?: return null
        val value = firstArgument(source) ?: return null
        if (value.tokenType != KtNodeTypes.STRING_TEMPLATE) return null
        val content = StringBuilder()
        for (part in lightChildren(source, value)) {
            when (part.tokenType) {
                KtTokens.OPEN_QUOTE, KtTokens.CLOSING_QUOTE -> Unit
                KtNodeTypes.LITERAL_STRING_TEMPLATE_ENTRY, KtNodeTypes.ESCAPE_STRING_TEMPLATE_ENTRY ->
                    content.append(source.treeStructure.toString(part))
                else -> return null
            }
        }
        return content.toString()
    }

    private fun significantChildren(source: KtSourceElement, node: LighterASTNode): List<LighterASTNode> =
        lightChildren(source, node).filter { it.tokenType != KtTokens.WHITE_SPACE && it.tokenType !in KtTokens.COMMENTS }

    // Go's weakKeySizeThreshold.
    private fun threshold(algorithm: String): Int? {
        val normalized = upperCodePoints(trimGoSpace(algorithm).replace("-", ""))
        return when {
            normalized == "RSA" || normalized == "DSA" -> 2048
            normalized == "EC" || normalized == "ECDSA" -> 224
            normalized == "AES" -> 128
            normalized.startsWith("HMACSHA") -> 256
            else -> null
        }
    }

    // Go's strings.TrimSpace trims runes where unicode.IsSpace holds.
    private fun trimGoSpace(text: String): String = text.trim(::isGoSpace)

    private fun isGoSpace(c: Char): Boolean = when (c) {
        '\t', '\n', '\u000B', '\u000C', '\r', ' ', '\u0085', '\u00A0',
        '\u1680', '\u2028', '\u2029', '\u202F', '\u205F', '\u3000' -> true
        else -> c in '\u2000'..'\u200A'
    }

    // Go's strings.ToUpper maps each code point to its simple uppercase form.
    private fun upperCodePoints(text: String): String {
        val out = StringBuilder(text.length)
        var i = 0
        while (i < text.length) {
            val cp = text.codePointAt(i)
            out.appendCodePoint(Character.toUpperCase(cp))
            i += Character.charCount(cp)
        }
        return out.toString()
    }
}
