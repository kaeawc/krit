package dev.jasonpearson.krit.fir.checkers.resourcecost

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.containingClassLookupTag
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirAnonymousObjectExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLoop
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirSuperReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirTypeOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirClassLikeSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneTypeOrNull
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isNothingOrNullableNothing
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.isUnit
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.Name
import java.util.IdentityHashMap

/**
 * Port of the Go BufferedReadWithoutBuffer rule: `read(...)` on an input
 * stream whose bytes come straight from a `java.io.FileInputStream`, with no
 * `BufferedInputStream` in between. Reported on the call, like Go.
 *
 * Mirrored from Go:
 * - the call is named `read` and has an explicit receiver (a bare `read()`,
 *   `this.read()` or `super.read()` is not reported, as Go finds no receiver
 *   evidence there);
 * - the receiver is a FileInputStream constructor call (`FileInputStream(p)
 *   .read()`), a local initialized with one (`val s: InputStream =
 *   FileInputStream(p)`), or an expression that wraps one (`DataInputStream(
 *   FileInputStream(p)).read()`, `(FileInputStream(p) as InputStream).read()`,
 *   `run { DataInputStream(FileInputStream(p)) }.read()`, `object :
 *   FilterInputStream(FileInputStream(p)) {}.read()`);
 * - a `buffered()` / `BufferedInputStream(...)` call among the stream's
 *   sources makes the read buffered, even when another source is a raw
 *   FileInputStream (`SequenceInputStream(BufferedInputStream(a), b)`,
 *   `if (c) FileInputStream(p) else BufferedInputStream(...)`), as Go exempts
 *   any receiver that calls one.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: the read must be a member `read` of a `java.io.InputStream`
 *   subtype, and the FileInputStream must be the source of the receiver
 *   stream. Go accepts any call named `read` whose receiver text contains a
 *   call named `FileInputStream`, so it also reports a Reader
 *   (`FileInputStream(p).reader().read()`, `.bufferedReader().read()`,
 *   `InputStreamReader(FileInputStream(p)).read()`), a FileChannel
 *   (`FileInputStream(p).channel.read(buffer)`), an in-memory stream built
 *   from the file's bytes (`ByteArrayInputStream(FileInputStream(p)
 *   .readBytes()).read()`), an extension function named `read`, a class of
 *   another package named FileInputStream, a BufferedInputStream subclass or
 *   anonymous object, and a local var whose FileInputStream is replaced (by a
 *   buffered or another stream) on every path before the read. None of those
 *   reads a FileInputStream unbuffered.
 * - Recall: the receiver's resolved type proves a FileInputStream where Go
 *   has no source evidence: a parameter or property typed FileInputStream (or
 *   a subclass), a lambda parameter (`FileInputStream(p).use { it.read() }`),
 *   `File.inputStream()`, a smart cast, an import or type alias, a local
 *   initialized with a wrapper, an if/when, a cast or a try expression, a
 *   local assigned a FileInputStream after its initializer, and a
 *   FileInputStream receiver whose chain calls `buffered()` off the stream
 *   path (`FileInputStream(p).also { it.buffered() }.read()`).
 */
internal object BufferedReadWithoutBuffer : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "BufferedReadWithoutBuffer"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(BufferedReadWithoutBuffer)
    }

    private const val MESSAGE =
        "FileInputStream.read() without BufferedInputStream; wrap in .buffered() for efficient reads."

    private val READ = Name.identifier("read")

    // The calls Go treats as buffering the receiver.
    private val BUFFERING_CALLS = setOf(Name.identifier("buffered"), Name.identifier("BufferedInputStream"))
    private val INPUT_STREAM = streamType("java/io/InputStream")
    private val FILE_INPUT_STREAM = streamType("java/io/FileInputStream")
    private val BUFFERED_INPUT_STREAM = streamType("java/io/BufferedInputStream")

    // Bounds the recursion through wrappers, local values and type arguments.
    private const val MAX_DEPTH = 32

    private fun streamType(id: String): ConeKotlinType =
        ClassId.fromString(id).constructClassLikeType(emptyArray(), isMarkedNullable = true)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return
        if (callee.name != READ) return
        // A member read of the stream, not an extension named read.
        if (callee.receiverParameterSymbol != null || callee.containingClassLookupTag() == null) return
        val receiver = expression.explicitReceiver ?: return
        if (receiver is FirThisReceiverExpression || receiver is FirSuperReceiverExpression) return
        if (!isSubtype(receiver.resolvedType, INPUT_STREAM)) return
        if (Flow().source(receiver) != Source.FILE) return
        report(expression.source, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isSubtype(type: ConeKotlinType, of: ConeKotlinType): Boolean = type.isSubtypeOf(of, context.session)

    // Where a stream's bytes come from. BUFFERED is a buffering call among
    // the sources, which Go treats as buffering the whole receiver.
    private enum class Source { NONE, FILE, BUFFERED }

    private fun combine(sources: Sequence<Source>): Source {
        var result = Source.NONE
        for (source in sources) {
            if (source == Source.BUFFERED) return Source.BUFFERED
            if (source == Source.FILE) result = Source.FILE
        }
        return result
    }

    // One receiver's evaluation: tracks the values being resolved so a cycle
    // through local initializers and assignments ends.
    private class Flow {
        private var layout: Layout? = null
        private val visiting = HashSet<FirExpression>()
        private var depth = 0

        // The source of [expression]: FILE when it is a FileInputStream (by
        // its resolved type), a stream built from one (a call that takes it
        // as its receiver, an argument or a lambda result, an anonymous
        // object whose superclass constructor takes it), a local holding
        // one, or a branch of an if/when/elvis/try that is one.
        context(context: CheckerContext)
        fun source(expression: FirExpression): Source {
            if (depth >= MAX_DEPTH) return Source.NONE
            depth++
            try {
                val type = expression.resolvedType
                // `return`, `throw`, `error()`, `TODO()` and `null` are typed
                // Nothing(?), a subtype of every stream type; none is a stream.
                if (type.isNothingOrNullableNothing) return Source.NONE
                if (isSubtype(type, FILE_INPUT_STREAM)) return Source.FILE
                if (isSubtype(type, BUFFERED_INPUT_STREAM)) {
                    val call = expression as? FirFunctionCall
                    return if (call != null && call.calleeReference.name in BUFFERING_CALLS) Source.BUFFERED else Source.NONE
                }
                return when (expression) {
                    is FirWrappedArgumentExpression -> source(expression.expression)
                    is FirSmartCastExpression -> source(expression.originalExpression)
                    is FirCheckedSafeCallSubject -> source(expression.originalReceiverRef.value)
                    is FirSafeCallExpression -> (expression.selector as? FirExpression)?.let { source(it) } ?: Source.NONE
                    is FirCheckNotNullCall -> combine(expression.argumentList.arguments.asSequence().map { source(it) })
                    is FirTypeOperatorCall ->
                        if (expression.operation == FirOperation.AS || expression.operation == FirOperation.SAFE_AS) {
                            combine(expression.argumentList.arguments.asSequence().map { source(it) })
                        } else {
                            Source.NONE
                        }
                    else -> if (carriesStream(type, 0)) sourceOfValue(expression) else Source.NONE
                }
            } finally {
                depth--
            }
        }

        // An input stream, or a value that holds or yields one: a
        // collection of streams (`listOf(s).map { ... }`) or a lambda.
        context(context: CheckerContext)
        private fun carriesStream(type: ConeKotlinType, level: Int): Boolean {
            if (isSubtype(type, INPUT_STREAM)) return true
            if (level >= 4) return false
            val expanded = type.lowerBoundIfFlexible().fullyExpandedType(context.session)
            return expanded.typeArguments.any { argument ->
                val argumentType = argument.type ?: return@any false
                !argumentType.isNothingOrNullableNothing && carriesStream(argumentType, level + 1)
            }
        }

        @OptIn(DirectDeclarationsAccess::class)
        context(context: CheckerContext)
        private fun sourceOfValue(expression: FirExpression): Source = when (expression) {
            is FirFunctionCall -> {
                val arguments = expression.argumentList.arguments.asSequence().flatMap {
                    if (it is FirVarargArgumentsExpression) it.arguments.asSequence() else sequenceOf(it)
                }
                combine((listOfNotNull(expression.explicitReceiver).asSequence() + arguments).map { source(it) })
            }
            is FirAnonymousFunctionExpression -> combine(lambdaResults(expression.anonymousFunction).map { source(it) })
            is FirAnonymousObjectExpression -> {
                val constructor = expression.anonymousObject.declarations.firstOrNull { it is FirConstructor } as? FirConstructor
                val arguments = constructor?.delegatedConstructor?.argumentList?.arguments.orEmpty()
                combine(arguments.asSequence().map { source(it) })
            }
            is FirPropertyAccessExpression -> sourceOfLocal(expression)
            is FirWhenExpression -> combine(expression.branches.asSequence().map { sourceOfResult(it.result) })
            is FirElvisExpression -> combine(sequenceOf(expression.lhs, expression.rhs).map { source(it) })
            is FirTryExpression ->
                combine((sequenceOf(expression.tryBlock) + expression.catches.asSequence().map { it.block }).map { sourceOfResult(it) })
            else -> Source.NONE
        }

        context(context: CheckerContext)
        private fun sourceOfResult(block: FirBlock): Source {
            val last = block.statements.lastOrNull() as? FirExpression ?: return Source.NONE
            return source(last)
        }

        // The values a lambda returns: its `return@label` results and its
        // last expression. A lambda whose result is Unit returns no stream.
        private fun lambdaResults(lambda: FirAnonymousFunction): Sequence<FirExpression> {
            if (lambda.returnTypeRef.coneTypeOrNull?.isUnit == true) return emptySequence()
            val body = lambda.body ?: return emptySequence()
            val results = ArrayList<FirExpression>()
            body.accept(object : FirVisitorVoid() {
                override fun visitElement(element: FirElement) {
                    if (element is FirReturnExpression && element.target.labeledElement == lambda) results += element.result
                    element.acceptChildren(this)
                }
            })
            val last = body.statements.lastOrNull()
            if (last is FirExpression && last !is FirReturnExpression) results += last
            return results.asSequence()
        }

        // A local variable: FILE when a value that can reach this access (its
        // initializer or a later assignment, as Layout decides) is one. Go's
        // buffering exemption covers only the receiver text it sees, so a
        // buffering call inside one of the local's values vetoes neither its
        // other values nor the receiver's other sources.
        context(context: CheckerContext)
        private fun sourceOfLocal(access: FirPropertyAccessExpression): Source {
            if (access.explicitReceiver != null) return Source.NONE
            val symbol = access.calleeReference.toResolvedCallableSymbol() as? FirPropertySymbol ?: return Source.NONE
            if (!symbol.isLocal) return Source.NONE
            val initializer = symbol.resolvedInitializer
            val values = if (symbol.isVal && initializer != null) {
                listOf(initializer)
            } else {
                val layout = layout ?: Layout.of()?.also { layout = it } ?: return Source.NONE
                layout.reaching(symbol, initializer, access)
            }
            return combine(
                values.asSequence().map { value ->
                    if (!visiting.add(value)) return@map Source.NONE
                    try {
                        source(value).takeUnless { it == Source.BUFFERED } ?: Source.NONE
                    } finally {
                        visiting -= value
                    }
                },
            )
        }
    }

    // Source offsets of an element.
    private class Span(val start: Int, val end: Int) {
        fun contains(offset: Int): Boolean = offset in start until end

        fun contains(other: Span): Boolean = other.start >= start && other.end <= end
    }

    private fun spanOf(element: FirElement): Span? {
        val source = element.source ?: return null
        return Span(source.startOffset, source.endOffset)
    }

    // An assignment to a local: the values it stores, where it is, and the
    // block it is a statement of (null when it sits inside an expression or
    // combines with the current value, `x += y`, so it replaces nothing).
    private class Assignment(val values: List<FirExpression>, val span: Span, val block: Span?)

    // The control structure of the outermost declaration enclosing the
    // checked call, enough to decide which values of a local can reach an
    // access of it: its assignments, loops, if/when branches, and nested
    // lambdas, functions and classes.
    private class Layout(
        private val assignments: Map<FirBasedSymbol<*>, List<Assignment>>,
        private val loops: List<Span>,
        private val branches: List<List<Span>>,
        private val nested: List<Span>,
    ) {
        // The initializer and assignments of [symbol] that can reach
        // [access]. An assignment reaches it when it can run before it,
        // directly or around a loop, and no later assignment that always runs
        // in between replaces it. An access inside a lambda, function or class
        // nested in the local's scope may run at any time, so every value
        // reaches it.
        fun reaching(symbol: FirPropertySymbol, initializer: FirExpression?, access: FirExpression): List<FirExpression> {
            val all = assignments[symbol].orEmpty()
            val declaration = symbol.source?.let { Span(it.startOffset, it.endOffset) }
            val at = spanOf(access)
            val everything = listOfNotNull(initializer) + all.flatMap { it.values }
            if (declaration == null || at == null) return everything
            if (nested.any { it.contains(at.start) && !it.contains(declaration.start) }) return everything
            val result = ArrayList<FirExpression>()
            if (initializer != null && reaches(declaration, at, declaration, all)) result += initializer
            for (assignment in all) {
                if (reaches(assignment.span, at, declaration, all)) result += assignment.values
            }
            return result
        }

        private fun reaches(definition: Span, at: Span, declaration: Span, all: List<Assignment>): Boolean {
            val forward = definition.end <= at.start &&
                !exclusive(definition, at) &&
                all.none { killer ->
                    killer.span.start >= definition.end && killer.span.end <= at.start &&
                        killer.block?.contains(at.start) == true
                }
            if (forward) return true
            return loops.any { loop ->
                loop.contains(definition) && loop.contains(at.start) && !loop.contains(declaration.start) &&
                    all.none { killer ->
                        val block = killer.block
                        block != null && loop.contains(block) && block.contains(at.start) && killer.span.end <= at.start
                    }
            }
        }

        // In different branches of one if/when, so only a loop connects them.
        private fun exclusive(definition: Span, at: Span): Boolean = branches.any { spans ->
            val from = spans.indexOfFirst { it.contains(definition) }
            val to = spans.indexOfFirst { it.contains(at.start) }
            from >= 0 && to >= 0 && from != to
        }

        companion object {
            @OptIn(SymbolInternals::class)
            context(context: CheckerContext)
            fun of(): Layout? {
                val rootSymbol = context.containingDeclarations
                    .firstOrNull { it !is FirClassLikeSymbol<*> && it !is FirFileSymbol } ?: return null
                val root = rootSymbol.fir
                val assignments = HashMap<FirBasedSymbol<*>, MutableList<Assignment>>()
                val blocks = IdentityHashMap<FirVariableAssignment, Span>()
                val loops = ArrayList<Span>()
                val branches = ArrayList<List<Span>>()
                val nested = ArrayList<Span>()
                root.accept(object : FirVisitorVoid() {
                    override fun visitElement(element: FirElement) {
                        when (element) {
                            is FirBlock -> {
                                val span = spanOf(element)
                                if (span != null) {
                                    element.statements.forEach { if (it is FirVariableAssignment) blocks[it] = span }
                                }
                            }
                            is FirLoop -> spanOf(element)?.let { loops += it }
                            is FirWhenExpression -> branches += element.branches.mapNotNull { spanOf(it.result) }
                            is FirVariableAssignment -> record(element)
                            is FirFunction, is FirClass -> if (element !== root) spanOf(element)?.let { nested += it }
                            else -> {}
                        }
                        element.acceptChildren(this)
                    }

                    private fun record(assignment: FirVariableAssignment) {
                        val span = spanOf(assignment) ?: return
                        val lValue = assignment.lValue
                        val compound = lValue is FirDesugaredAssignmentValueReferenceExpression
                        val target = (if (compound) lValue.expressionRef.value else lValue) as? FirQualifiedAccessExpression
                        val symbol = target?.calleeReference?.toResolvedCallableSymbol() ?: return
                        val value = assignment.rValue
                        // `x += y` combines y with the current value.
                        val values = if (compound && value is FirFunctionCall) value.argumentList.arguments else listOf(value)
                        assignments.getOrPut(symbol) { ArrayList() } += Assignment(values, span, if (compound) null else blocks[assignment])
                    }
                })
                return Layout(assignments, loops, branches, nested)
            }
        }
    }
}
