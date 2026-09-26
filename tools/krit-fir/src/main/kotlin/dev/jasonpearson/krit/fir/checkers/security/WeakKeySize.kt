package dev.jasonpearson.krit.fir.checkers.security

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.significantChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.containingClassLookupTag
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.impl.FirDefaultPropertyGetter
import org.jetbrains.kotlin.fir.declarations.utils.isConst
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirComponentCall
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirResolvedQualifier
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirStringConcatenationCall
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirConstructorSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirReceiverParameterSymbol
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
 * `KeyGenerator.getInstance(<alg>)` or `KeyPairGenerator.getInstance(<alg>)`
 * with a string literal or `const val` algorithm.
 *
 * Mirrors the Go rule's evidence on top of FIR resolution:
 * - the call must resolve to KeyGenerator.init or KeyPairGenerator.initialize
 *   (any overload), with a bare variable as its receiver (`gen.init(64)`,
 *   `gen?.init(64)`, a smart-cast `gen`);
 * - the size is the first argument's source text, read as Go reads it: a
 *   decimal integer with an optional sign, `_` separators, and an `L`/`l`
 *   suffix, so a hex literal or a constant never counts;
 * - the algorithms come from the writes to that variable (its initializer, an
 *   assignment, a destructuring entry) that complete before the call. A write
 *   that runs on every path to the call replaces the earlier ones only when
 *   its algorithm is known on every path through its value; a write whose
 *   algorithm is unknown (`g = cached ?: g`, `g = KG(name)`) may keep the old
 *   generator, so it replaces nothing;
 * - a write's algorithm is that of the getInstance call producing its value:
 *   the last statement of a run/with/let lambda, the receiver of also/apply,
 *   every branch of if/when/try/elvis, a destructuring entry's own Pair,
 *   Triple, or `to` operand. Any other value takes, as Go does, the first
 *   getInstance call inside it whose nearest enclosing write is that write;
 * - the algorithm is a string literal's source text (escape entries kept, as
 *   Go reads it) or a `const val`'s value, normalized as Go does: surrounding
 *   whitespace trimmed with Go's unicode.IsSpace set, `-` removed, uppercased.
 * The writes searched are those of the declaration that holds every write of
 * a local variable (the outermost enclosing function, property initializer, or
 * init block), and, for a member or top-level property, those of the nearest
 * enclosing named function (else the nearest enclosing constructor, init
 * block, accessor, or property initializer) made on the object the call reads
 * the property from (the same class `this`, object, variable, or scope-function
 * receiver). A final member or top-level `val` with an initializer and a
 * default getter takes the algorithm of its initializer. When the variable's
 * own writes name no algorithm (a lambda parameter, a reassignment with an
 * unknown algorithm), the checker reads the generator as Go does: the first
 * literal getInstance before the call in the nearest named function whose
 * write writes a variable of the same name.
 *
 * Deliberate differences from Go, pinned by goldens. Go matches the variable
 * by name and the getInstance receiver by spelling; FIR resolves both:
 * - Recall (Go misses these true positives, WeakKeySizeGoMisses,
 *   WeakKeySizeDeclaredName, WeakKeySizeCrossFileTest): the getInstance
 *   receiver spelled through an import alias, a typealias, or parentheses, or
 *   in a file that also declares an unrelated class named KeyGenerator or
 *   KeyPairGenerator; a `const val` algorithm; a write outside Go's enclosing
 *   function_declaration (a local variable used in a local function, local
 *   class or anonymous object method, a variable in an init block, property
 *   initializer or getter, or constructor parameter default); a final member
 *   or top-level `val` initialized with getInstance, in the file or another;
 *   a `when` subject variable; a later conditional write whose algorithm makes
 *   the size weak; a variable named like one declared earlier in the
 *   function; a destructuring entry whose own operand is weak; an annotated,
 *   labeled, or comment-led argument; a parenthesized or backticked receiver.
 * - Precision (Go reports these, but the generator does not hold the
 *   algorithm Go reads, WeakKeySizePrecision and WeakKeySizeMemberWrites):
 *   another variable sharing a getInstance variable's name; a variable
 *   reassigned on every path before the call; a value produced by another
 *   getInstance than its first (a run block, a destructuring entry); a
 *   member written on another object; a receiver that is not a KeyGenerator
 *   or KeyPairGenerator; a getInstance call on a local that shadows the
 *   KeyGenerator import.
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
        val access = variableReceiver(expression.explicitReceiver) ?: return
        val symbol = access.calleeReference.toResolvedCallableSymbol() as? FirVariableSymbol<*> ?: return
        val size = firstArgumentSize(source) ?: return
        val algorithms = algorithmsReaching(symbol, access, source.startOffset)
        val weak = algorithms.any { algorithm ->
            val threshold = threshold(algorithm)
            threshold != null && size < threshold
        }
        if (weak) report(source, MESSAGE)
    }

    // The access to the variable a bare-identifier receiver names: `gen`,
    // `gen?.`, or a smart-cast `gen`, a property or a Java field. Go takes
    // only a receiver without a dot.
    private fun variableReceiver(receiver: FirExpression?): FirPropertyAccessExpression? {
        val access = unwrap(receiver) as? FirPropertyAccessExpression ?: return null
        if (access.explicitReceiver != null) return null
        return access
    }

    private fun unwrap(expression: FirExpression?): FirExpression? {
        var current = expression
        while (true) {
            current = when (current) {
                is FirCheckedSafeCallSubject -> current.originalReceiverRef.value
                is FirSmartCastExpression -> current.originalExpression
                else -> return current
            }
        }
    }

    // The algorithms [symbol] may hold at [callStart]. When the variable's own
    // writes name none, falls back to Go's reading by name.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun algorithmsReaching(
        symbol: FirVariableSymbol<*>,
        access: FirPropertyAccessExpression,
        callStart: Int,
    ): List<String> {
        val own = if (symbol is FirPropertySymbol && !isLocal(symbol) && isFinalInitializedVal(symbol)) {
            val writes = Writes().also { symbol.fir.accept(it) }
            writes.of(symbol, local = false, owner = null).firstOrNull()?.let { writes.value(it).algorithms }.orEmpty()
        } else {
            writtenAlgorithms(symbol, access, callStart)
        }
        return own.ifEmpty { listOfNotNull(goAlgorithmByName(symbol.name, callStart)) }
    }

    // A final `val` whose every read yields its initializer's value.
    @OptIn(SymbolInternals::class)
    private fun isFinalInitializedVal(symbol: FirPropertySymbol): Boolean {
        val property = symbol.fir
        return symbol.isVal && property.initializer != null && property.delegate == null &&
            (property.getter == null || property.getter is FirDefaultPropertyGetter) &&
            symbol.resolvedStatus.modality == Modality.FINAL
    }

    // The algorithms of the writes to [symbol] that complete before
    // [callStart] and are not replaced, on every path, by a later write whose
    // algorithm is known.
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun writtenAlgorithms(
        symbol: FirVariableSymbol<*>,
        access: FirPropertyAccessExpression,
        callStart: Int,
    ): List<String> {
        val declarations = context.containingDeclarations
        val local = isLocal(symbol)
        val region = if (local) {
            declarations.firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol }
        } else {
            declarations.lastOrNull { it is FirNamedFunctionSymbol }
                ?: declarations.lastOrNull {
                    it !is FirClassLikeSymbol<*> && it !is FirFileSymbol && it !is FirAnonymousFunctionSymbol
                }
        } ?: return emptyList()
        val writes = Writes().also { region.fir.accept(it) }
        val owner = if (local) null else writes.ownerOf(access)
        // A write counts once it has completed: a call inside its own value
        // (`gen = KG("AES").also { gen.init(128) }`) still sees the old value.
        val before = writes.of(symbol, local, owner).filter { it.end <= callStart }.sortedBy { it.start }
        val values = before.map { writes.value(it) }
        // A write that runs on every path to the call, with a known
        // algorithm on every path through its value, replaces the ones
        // before it. A write whose algorithm is unknown (`g = cached ?: g`,
        // `g = checkNotNull(g)`, `g = KG(name)`) may keep the old generator.
        val last = before.indices.lastOrNull { before[it].dominates(callStart) && values[it].exact }
        return values.drop(last ?: 0).flatMap { it.algorithms }
    }

    // Go's reading: the first getInstance call with a string literal
    // algorithm before [callStart] in the nearest named function whose
    // nearest enclosing write writes a variable named [name].
    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun goAlgorithmByName(name: Name, callStart: Int): String? {
        val function = context.containingDeclarations.lastOrNull { it is FirNamedFunctionSymbol } ?: return null
        val writes = Writes().also { function.fir.accept(it) }
        return writes.firstLiteralNamed(name, callStart)
    }

    // A local variable or a parameter: its writes all lie in its declaring
    // function, and its callable id does not identify it.
    private fun isLocal(symbol: FirVariableSymbol<*>): Boolean =
        (symbol is FirPropertySymbol && symbol.isLocal) || symbol is FirValueParameterSymbol

    // A getInstance call attributed to its nearest enclosing write: [literal]
    // is the string literal algorithm as Go reads it, [known] that or the
    // value of a `const val` argument.
    private class Instance(val start: Int, val literal: String?, val known: String)

    // The algorithms a write's value may hold; [exact] when every path
    // through the value ends in a getInstance call with a known algorithm.
    private class Value(val algorithms: List<String>, val exact: Boolean)

    private val unknown = Value(emptyList(), exact = false)

    private fun union(values: List<Value>): Value =
        Value(values.flatMap { it.algorithms }, values.isNotEmpty() && values.all { it.exact })

    private class Write(
        val target: FirBasedSymbol<*>,
        val name: Name?,
        val start: Int,
        val end: Int,
        private val block: KtSourceElement?,
        // The initializer or the assigned value.
        val value: FirExpression?,
        // The assigned property access, for an assignment.
        val access: FirQualifiedAccessExpression?,
    ) {
        val instances = ArrayList<Instance>()

        // For a destructuring entry, the whole declaration's write and the
        // entry's component number.
        var aliasOf: Write? = null
        var component = 0

        // True when the write is a statement of a block that contains [offset].
        fun dominates(offset: Int): Boolean = block != null && block.startOffset <= offset && offset < block.endOffset
    }

    private val kotlinPackage = FqName("kotlin")
    private val returningScopes = setOf("run", "with", "let").mapTo(HashSet()) { CallableId(kotlinPackage, Name.identifier(it)) }
    private val selfScopes = setOf("also", "apply").mapTo(HashSet()) { CallableId(kotlinPackage, Name.identifier(it)) }
    private val receiverScopes = setOf("run", "with", "apply").mapTo(HashSet()) { CallableId(kotlinPackage, Name.identifier(it)) }
    private val applyId = CallableId(kotlinPackage, Name.identifier("apply"))
    private val withId = CallableId(kotlinPackage, Name.identifier("with"))
    private val toId = CallableId(kotlinPackage, Name.identifier("to"))
    private val tupleClasses = setOf(
        ClassId(kotlinPackage, Name.identifier("Pair")),
        ClassId(kotlinPackage, Name.identifier("Triple")),
    )
    private val tupleParameters = listOf("first", "second", "third")

    // No dispatch or extension receiver: a top-level property.
    private val noOwner = Any()

    // Collects every write in a declaration, attributing each qualifying
    // getInstance call to its nearest enclosing write.
    private class Writes : FirVisitorVoid() {
        private val all = ArrayList<Write>()
        private val blockOf = HashMap<FirElement, KtSourceElement?>()
        private val destructs = HashMap<FirBasedSymbol<*>, Write>()

        // The receiver a lambda passed to with/run/apply binds as `this`.
        private val lambdaOwners = HashMap<FirAnonymousFunctionSymbol, FirExpression>()
        private var current: Write? = null

        // The writes to [symbol]; a member or top-level property or a field
        // also matches through another symbol for the same declaration (a
        // fake override, a synthetic Java property). An assignment to a
        // member counts only on the object the call reads it from ([owner]).
        fun of(symbol: FirVariableSymbol<*>, local: Boolean, owner: Any?): List<Write> = all.filter {
            val same = it.target == symbol ||
                (!local && it.target is FirVariableSymbol<*> && !isLocal(it.target) && it.target.callableId == symbol.callableId)
            same && (local || owner == null || it.access == null || ownerOf(it.access) == owner)
        }

        // Go's algorithm for a variable named [name] (see goAlgorithmByName).
        fun firstLiteralNamed(name: Name, callStart: Int): String? = all
            .filter { it.name == name }
            .flatMap { (it.aliasOf ?: it).instances }
            .filter { it.literal != null && it.start < callStart }
            .minByOrNull { it.start }?.literal

        // The object a member access reads the property from: a class (its
        // `this`), an object, a variable, or a scope-function lambda whose
        // receiver is none of these.
        fun ownerOf(access: FirQualifiedAccessExpression): Any {
            val receiver = access.dispatchReceiver ?: access.extensionReceiver ?: return noOwner
            return keyOf(receiver, 0)
        }

        @OptIn(SymbolInternals::class)
        private fun keyOf(expression: FirExpression, depth: Int): Any {
            val value = unwrap(expression) ?: return expression
            if (depth > 8) return value
            return when (value) {
                is FirThisReceiverExpression -> {
                    when (val bound = value.calleeReference.boundSymbol) {
                        is FirClassLikeSymbol<*> -> bound
                        is FirReceiverParameterSymbol -> {
                            val owner = bound.containingDeclarationSymbol
                            if (owner is FirAnonymousFunctionSymbol) lambdaKey(owner, depth) else owner
                        }
                        is FirAnonymousFunctionSymbol -> lambdaKey(bound, depth)
                        null -> value
                        else -> bound
                    }
                }
                is FirResolvedQualifier -> value.symbol ?: value
                is FirPropertyAccessExpression -> {
                    val symbol = value.calleeReference.toResolvedCallableSymbol() as? FirVariableSymbol<*>
                    if (symbol == null || value.explicitReceiver != null) return value
                    // `val b = Holder().apply { gen = ... }`: b is the object
                    // the apply lambda wrote.
                    if (symbol is FirPropertySymbol && symbol.isLocal && symbol.isVal) {
                        applyLambda(symbol.fir.initializer)?.let { return lambdaKey(it, depth + 1) }
                    }
                    symbol
                }
                else -> applyLambda(value)?.let { lambdaKey(it, depth + 1) } ?: value
            }
        }

        private fun lambdaKey(lambda: FirAnonymousFunctionSymbol, depth: Int): Any {
            val owner = lambdaOwners[lambda] ?: return lambda
            return if (isVariableLike(owner)) keyOf(owner, depth + 1) else lambda
        }

        private fun isVariableLike(expression: FirExpression): Boolean = when (val value = unwrap(expression)) {
            is FirThisReceiverExpression, is FirResolvedQualifier -> true
            is FirPropertyAccessExpression -> value.explicitReceiver == null
            else -> false
        }

        private fun applyLambda(expression: FirExpression?): FirAnonymousFunctionSymbol? {
            val call = unwrap(expression) as? FirFunctionCall ?: return null
            if (call.calleeReference.toResolvedCallableSymbol()?.callableId != applyId) return null
            return lambdaArgument(call)?.symbol
        }

        // The algorithms [write]'s value may hold.
        fun value(write: Write): Value {
            val destruct = write.aliasOf
            if (destruct != null) {
                val operand = componentOperand(destruct.value, write.component)
                val traced = operand?.let { trace(it, destruct, 0) } ?: unknown
                return traced.takeIf { it.algorithms.isNotEmpty() } ?: attributed(destruct.value, destruct)
            }
            val traced = trace(write.value, write, 0)
            // Nothing traced: take the first getInstance in the value, as Go does.
            return traced.takeIf { it.algorithms.isNotEmpty() } ?: attributed(write.value, write)
        }

        // The Pair/Triple constructor argument or `to` operand a
        // destructuring entry takes, or null for any other value.
        private fun componentOperand(value: FirExpression?, component: Int): FirExpression? {
            val call = unwrap(value) as? FirFunctionCall ?: return null
            val callee = call.calleeReference.toResolvedCallableSymbol() ?: return null
            if (callee is FirConstructorSymbol) {
                if (callee.containingClassLookupTag()?.classId !in tupleClasses) return null
                val parameter = tupleParameters.getOrNull(component - 1) ?: return null
                return call.resolvedArgumentMapping?.entries?.firstOrNull { it.value.name.asString() == parameter }?.key
            }
            if (callee.callableId != toId) return null
            return when (component) {
                1 -> call.explicitReceiver ?: call.extensionReceiver
                2 -> call.argumentList.arguments.firstOrNull()
                else -> null
            }
        }

        // The algorithms of the getInstance calls that produce [expression]'s
        // value: the last statement of a run/with/let lambda, the receiver
        // of also/apply, every branch of if/when/try/elvis. Any other
        // expression takes its first attributed getInstance, as Go does.
        private fun trace(expression: FirExpression?, write: Write, depth: Int): Value {
            if (expression == null) return unknown
            if (depth > 32) return attributed(expression, write)
            val next = depth + 1
            return when (expression) {
                is FirSmartCastExpression -> trace(expression.originalExpression, write, next)
                is FirCheckedSafeCallSubject -> trace(expression.originalReceiverRef.value, write, next)
                is FirSafeCallExpression -> trace(expression.selector as? FirExpression, write, next)
                is FirCheckNotNullCall -> trace(expression.argumentList.arguments.firstOrNull(), write, next)
                is FirTypeOperatorCall ->
                    if (expression.operation == FirOperation.AS || expression.operation == FirOperation.SAFE_AS) {
                        trace(expression.argumentList.arguments.firstOrNull(), write, next)
                    } else {
                        attributed(expression, write)
                    }
                is FirReturnExpression -> trace(expression.result, write, next)
                is FirBlock -> trace(expression.statements.lastOrNull() as? FirExpression, write, next)
                is FirWhenExpression -> union(expression.branches.map { trace(it.result, write, next) })
                is FirTryExpression ->
                    union((listOf(expression.tryBlock) + expression.catches.map { it.block }).map { trace(it, write, next) })
                is FirElvisExpression -> union(listOf(trace(expression.lhs, write, next), trace(expression.rhs, write, next)))
                is FirComponentCall -> attributed(expression, write)
                is FirFunctionCall -> {
                    val id = expression.calleeReference.toResolvedCallableSymbol()?.callableId
                    when (id) {
                        in getInstanceIds -> instanceAlgorithm(expression)?.let { Value(listOf(it), exact = true) } ?: unknown
                        in returningScopes -> {
                            val lambda = lambdaArgument(expression)
                            if (lambda == null) attributed(expression, write) else trace(lambda.body, write, next)
                        }
                        in selfScopes -> trace(expression.explicitReceiver ?: expression.extensionReceiver, write, next)
                        else -> attributed(expression, write)
                    }
                }
                else -> attributed(expression, write)
            }
        }

        // The first getInstance call attributed to [write] inside [expression].
        private fun attributed(expression: FirExpression?, write: Write): Value {
            val source = expression?.source ?: return unknown
            val first = write.instances
                .filter { it.start >= source.startOffset && it.start < source.endOffset }
                .minByOrNull { it.start } ?: return unknown
            return Value(listOf(first.known), exact = false)
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
                    val source = element.source
                    val write = Write(
                        element.symbol, element.name, source?.startOffset ?: Int.MAX_VALUE,
                        source?.endOffset ?: Int.MAX_VALUE, blockOf[element], initializer, null,
                    )
                    all += write
                    if (element.name == SpecialNames.DESTRUCT) destructs[element.symbol] = write
                    if (initializer is FirComponentCall) {
                        val subject = (initializer.explicitReceiver as? FirQualifiedAccessExpression)
                            ?.calleeReference?.toResolvedCallableSymbol()
                        write.aliasOf = subject?.let { destructs[it] }
                        write.component = initializer.componentIndex
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
                    val source = element.source
                    val write = Write(
                        symbol, (symbol as? FirVariableSymbol<*>)?.name, source?.startOffset ?: Int.MAX_VALUE,
                        source?.endOffset ?: Int.MAX_VALUE, blockOf[element], element.rValue, target,
                    )
                    all += write
                    lValue.accept(this)
                    within(write) { element.rValue.accept(this) }
                }
                is FirFunctionCall -> {
                    val id = element.calleeReference.toResolvedCallableSymbol()?.callableId
                    val write = current
                    val start = element.source?.startOffset
                    if (write != null && start != null && id in getInstanceIds) {
                        val literal = firstArgumentString(element.source)
                        val known = literal ?: constantString(element.argumentList.arguments.firstOrNull(), 0)
                        if (known != null) write.instances += Instance(start, literal, known)
                    }
                    if (id in receiverScopes) {
                        val owner = if (id == withId) {
                            element.argumentList.arguments.firstOrNull()
                        } else {
                            element.explicitReceiver ?: element.extensionReceiver
                        }
                        val lambda = lambdaArgument(element)
                        if (owner != null && lambda != null) lambdaOwners[lambda.symbol] = owner
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

    private fun lambdaArgument(call: FirFunctionCall): FirAnonymousFunction? =
        (call.argumentList.arguments.lastOrNull() as? FirAnonymousFunctionExpression)?.anonymousFunction

    // The algorithm of a getInstance call: its string literal first
    // argument as Go reads it, or the value of a `const val` argument.
    private fun instanceAlgorithm(call: FirFunctionCall): String? =
        firstArgumentString(call.source) ?: constantString(call.argumentList.arguments.firstOrNull(), 0)

    // The value of a reference to a string `const val`.
    @OptIn(SymbolInternals::class)
    private fun constantString(expression: FirExpression?, depth: Int): String? {
        val access = unwrap(expression) as? FirPropertyAccessExpression ?: return null
        val symbol = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return null
        if (!symbol.isConst || depth > 8) return null
        return constantValue(symbol.resolvedInitializer, depth + 1)
    }

    private fun constantValue(expression: FirExpression?, depth: Int): String? = when (val value = unwrap(expression)) {
        is FirLiteralExpression -> value.value as? String
        is FirStringConcatenationCall -> value.argumentList.arguments
            .map { constantValue(it, depth) ?: return null }
            .joinToString("")
        else -> constantString(value, depth)
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
