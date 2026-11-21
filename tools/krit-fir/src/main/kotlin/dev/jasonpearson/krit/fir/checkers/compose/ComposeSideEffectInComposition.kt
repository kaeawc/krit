package dev.jasonpearson.krit.fir.checkers.compose

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.assignedAccess
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.expandedBound
import dev.jasonpearson.krit.fir.support.hasTypeAnnotation
import dev.jasonpearson.krit.fir.support.isNonNothingSubtypeOf
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.nullableStarProjectedType
import dev.jasonpearson.krit.fir.support.precedingComments
import dev.jasonpearson.krit.fir.support.selectorCallExpression
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapDelegate
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import dev.jasonpearson.krit.fir.support.writtenArgumentNameThroughWrappers
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.isObject
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFunctionChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.hasAnnotation
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.text

internal object ComposeSideEffectInComposition : FirFunctionChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ComposeSideEffectInComposition"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val functionCheckers = setOf(ComposeSideEffectInComposition)
    }

    private val composable = classIdOf("androidx.compose.runtime", "Composable")
    private val mutableState = classIdOf("androidx.compose.runtime", "MutableState").nullableStarProjectedType(1)
    private val mutableIntStateId = classIdOf("androidx.compose.runtime", "MutableIntState")
    private val mutableFloatStateId = classIdOf("androidx.compose.runtime", "MutableFloatState")
    private val transitionState = classIdOf("androidx.compose.animation.core", "MutableTransitionState").nullableStarProjectedType(1)
    private val idempotentPhrases = listOf("idempotent", "eager", "safe to repeat", "safe to run every")
    private val deferredLambdaCalls = setOf(
        "LaunchedEffect", "SideEffect", "DisposableEffect", "clickable", "combinedClickable", "pointerInput",
        "launch", "async", "toggleable", "selectable", "draggable", "scrollable",
    )
    private val compoundOperators = setOf("+=", "-=", "*=", "/=", "%=")
    private val plainFactories = setOf(
        "mutableMapOf", "mutableListOf", "mutableSetOf", "arrayListOf", "hashMapOf", "linkedMapOf", "arrayOf",
        "HashMap", "ArrayList", "LinkedHashMap", "IntArray", "BooleanArray", "ByteArray", "CharArray", "ShortArray",
        "FloatArray", "LongArray", "DoubleArray", "StringBuilder",
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFunction) {
        if (declaration !is FirNamedFunction && declaration !is FirAnonymousFunction) return
        if (declaration is FirAnonymousFunction && context.containingElements.any { (it is FirNamedFunction || it is FirAnonymousFunction) && it !== declaration }) return
        if (isTestSupportSource()) return
        val className = context.containingElements.filterIsInstance<FirRegularClass>()
            .lastOrNull { !it.classKind.isObject }?.name?.asString().orEmpty()
        val objectName = context.containingElements.filterIsInstance<FirRegularClass>().lastOrNull()?.name?.asString().orEmpty()
        if (className.startsWith("Fake") || "Test" in className || objectName.startsWith("Fake") || "Test" in objectName ||
            declaration.isCompositionPreview(previewConfig())) return
        val evidence = HashMap<FirFunction, BodyEvidence>()
        val enclosing = context.containingElements.toMutableList()
        declaration.body?.accept(object : FirVisitorVoid() {
            init {
                enclosing.add(declaration)
                if (isComposable(declaration, enclosing)) checkBody(declaration, captureFunctions(enclosing), evidence.getOrPut(declaration) { collectBody(declaration) })
            }

            override fun visitElement(element: FirElement) {
                if (element is FirNamedFunction || element is FirRegularClass) return
                enclosing.add(element)
                if (element is FirAnonymousFunction && isComposable(element, enclosing)) {
                    val functions = captureFunctions(enclosing)
                    checkBody(element, functions, evidence.getOrPut(declaration) { collectBody(declaration) })
                }
                element.acceptChildren(this)
                enclosing.removeAt(enclosing.lastIndex)
            }
        })
    }

    private class BodyEvidence(
        val locals: Map<FirCallableSymbol<*>, FirProperty>,
        val reads: Map<FirCallableSymbol<*>, Int>,
    )

    context(context: CheckerContext)
    private fun captureFunctions(enclosing: List<FirElement>): List<FirFunction> {
        val functions = enclosing.filterIsInstance<FirFunction>().asReversed()
        val owner = functions.indexOfFirst { it is FirNamedFunction && isComposable(it, enclosing) }
        return if (owner < 0) functions else functions.take(owner + 1)
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun checkBody(declaration: FirFunction, functions: List<FirFunction>, evidence: BodyEvidence) {
        val statements = declaration.body?.statements.orEmpty().map {
            if (it is FirReturnExpression && it.source?.kind is KtFakeSourceElementKind) it.result else it
        }
        for (statement in statements) {
            val source = statement.source
            if (source != null) {
                val operator = lightChildren(source, source.lighterASTNode)
                    .firstOrNull { it.tokenType == KtNodeTypes.OPERATION_REFERENCE }?.let { lightText(source, it) }
                val compound = operator in compoundOperators
                var mutation = if (statement is FirSafeCallExpression) statement.selector else statement
                while (compound && mutation is FirBlock) {
                    mutation = mutation.statements.lastOrNull() ?: break
                }
                when (mutation) {
                    is FirVariableAssignment -> checkWrite(source, assignedAccess(mutation), mutation.rValue, functions, evidence.locals, evidence.reads, false, compound)
                    is FirFunctionCall -> {
                        val callee = mutation.calleeReference.toResolvedCallableSymbol()
                        if (callee?.name?.asString() == "set" && (operator == "=" || compound)) {
                            checkWrite(source, mutation, mutation.argumentList.arguments.lastOrNull(), functions, evidence.locals, evidence.reads, true, compound)
                        } else if (compound && callee is FirFunctionSymbol<*> && callee.resolvedStatus.isOperator) {
                            checkWrite(source, mutation, mutation.argumentList.arguments.lastOrNull(), functions, evidence.locals, evidence.reads, false, true, true)
                        }
                    }
                    else -> Unit
                }
            }
        }
    }

    context(context: CheckerContext)
    private fun isComposable(function: FirFunction, enclosing: List<FirElement>): Boolean {
        if (function.hasAnnotation(composable, context.session)) return true
        if (function !is FirAnonymousFunction) return false
        if (function.typeRef.coneType.expandedBound().hasTypeAnnotation(composable)) return true
        return enclosing.any { owner ->
            when (owner) {
                is FirProperty ->
                    (unwrapSmartCasts(owner.initializer) as? FirAnonymousFunctionExpression)?.anonymousFunction === function &&
                        owner.returnTypeRef.coneType.expandedBound().hasTypeAnnotation(composable)
                is FirFunctionCall -> owner.resolvedArgumentMapping?.any { (argument, parameter) ->
                    (unwrapArgument(argument) as? FirAnonymousFunctionExpression)?.anonymousFunction === function &&
                        parameter.returnTypeRef.coneType.expandedBound().hasTypeAnnotation(composable)
                } == true
                else -> false
            }
        }
    }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    private fun checkWrite(
        source: KtSourceElement,
        target: FirQualifiedAccessExpression?,
        value: FirExpression?,
        functions: List<FirFunction>,
        locals: Map<FirCallableSymbol<*>, FirProperty>,
        reads: Map<FirCallableSymbol<*>, Int>,
        indexed: Boolean,
        compound: Boolean,
        mutatingCall: Boolean = false,
    ) {
        target ?: return
        val symbol = target.calleeReference.toResolvedCallableSymbol() ?: return
        val receiver = unwrapSmartCasts(target.explicitReceiver ?: target.dispatchReceiver ?: target.extensionReceiver, throughNotNull = true, throughSafeCallSubject = true)
        val receiverAccess = receiver as? FirQualifiedAccessExpression
        val receiverSymbol = receiverAccess?.calleeReference?.toResolvedCallableSymbol()
        val rootSymbol = localReceiver(receiver, locals) ?: receiverSymbol
        val local = locals[if (indexed || receiverSymbol != null) rootSymbol else symbol]
        val remembered = local?.initializer?.let(::isRemember) == true || local?.delegate?.let(::isRemember) == true
        val stateValue = !indexed && (
            symbol.name.asString() == "value" && receiver?.resolvedType?.isNonNothingSubtypeOf(mutableState) == true ||
                symbol.callableId?.classId == mutableIntStateId && symbol.name.asString() == "intValue" ||
                symbol.callableId?.classId == mutableFloatStateId && symbol.name.asString() == "floatValue"
            )
        val afterRead = stateValue && remembered && receiverSymbol == rootSymbol && (reads[receiverSymbol] ?: Int.MAX_VALUE) < source.startOffset
        if (remembered) {
            if (receiverSymbol != null && receiverSymbol != rootSymbol && !compound) return
            val root = receiverSymbol ?: symbol
            if (!compound && (indexed || receiverSymbol == null || symbol.name.asString() == "value") && (reads[root] ?: Int.MAX_VALUE) >= source.startOffset) return
            if (!indexed && !mutatingCall && receiverSymbol != null && !afterRead && (!compound || !stateValue)) return
        }
        if (local != null && !remembered && local.delegate == null) {
            if (!indexed && receiverSymbol == null) return
            if ((indexed || mutatingCall) && isPlainFactory(local.initializer)) return
        }
        if (!compound && symbol.name.asString() == "targetState" && receiver?.resolvedType?.isNonNothingSubtypeOf(transitionState) == true) return
        val rhsSymbol = (unwrapSmartCasts(value) as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()
        if (!compound && symbol.name.asString().startsWith("last") && receiverSymbol == null && functions.any { function -> function.valueParameters.any { it.symbol == rhsSymbol } }) return
        if (precedingComments(source).any { comment -> idempotentPhrases.any { it in comment.lowercase() } }) return
        val explicitReceiver = receiverAccess?.explicitReceiver
        val simpleStateReceiver = receiverSymbol != null && (explicitReceiver == null || explicitReceiver is FirThisReceiverExpression)
        if (isComposeInitializer(value) || stateValue && simpleStateReceiver && !afterRead && !compound) return
        val lhs = source.text.toString().substringBefore('=').trim()
        val subject = when {
            ".current" in lhs -> "state write through a CompositionLocal (`$lhs`)"
            lhs.endsWith(".value") -> "MutableState write (`$lhs`)"
            lhs.endsWith("]") -> "state map write (`$lhs`)"
            else -> "assignment"
        }
        report(source, "$subject inside a @Composable function body runs on every recomposition; wrap the mutation in LaunchedEffect / SideEffect / DisposableEffect so it only runs when intended.")
    }

    private fun localReceiver(expression: FirExpression?, locals: Map<FirCallableSymbol<*>, FirProperty>): FirCallableSymbol<*>? {
        var receiver = unwrapSmartCasts(expression, throughNotNull = true, throughSafeCallSubject = true)
        while (receiver is FirQualifiedAccessExpression && receiver !is FirFunctionCall) {
            val symbol = receiver.calleeReference.toResolvedCallableSymbol()
            if (symbol in locals) return symbol
            receiver = unwrapSmartCasts(receiver.explicitReceiver ?: receiver.dispatchReceiver ?: receiver.extensionReceiver, throughNotNull = true, throughSafeCallSubject = true)
        }
        return null
    }

    private fun callSymbol(expression: FirExpression?): FirCallableSymbol<*>? =
        (expression?.let { unwrapSmartCasts(unwrapDelegate(it)) } as? FirFunctionCall)?.calleeReference?.toResolvedCallableSymbol()

    private fun isRemember(expression: FirExpression): Boolean {
        val id = callSymbol(expression)?.callableId ?: return false
        return id.classId == null &&
            (id.packageName.asString() == "androidx.compose.runtime" && id.callableName.asString() == "remember" ||
                id.packageName.asString() == "androidx.compose.runtime.saveable" && id.callableName.asString() == "rememberSaveable")
    }

    private fun isComposeInitializer(expression: FirExpression?): Boolean {
        val access = expression?.let { unwrapSmartCasts(it) } as? FirQualifiedAccessExpression
        val id = (callSymbol(expression) ?: callSymbol(access?.explicitReceiver))?.callableId ?: return false
        val name = id.callableName.asString()
        return id.packageName.asString().startsWith("androidx.compose.") && (name.startsWith("remember") || name.startsWith("animate"))
    }

    private fun isPlainFactory(expression: FirExpression?): Boolean {
        val id = callSymbol(expression)?.callableId ?: return false
        return id.packageName.asString() in setOf("kotlin", "kotlin.collections", "kotlin.text", "java.util", "java.lang") &&
            (id.classId?.shortClassName?.asString() ?: id.callableName.asString()) in plainFactories
    }

    private fun collectBody(function: FirFunction): BodyEvidence {
        val locals = LinkedHashMap<FirCallableSymbol<*>, FirProperty>()
        val reads = HashMap<FirCallableSymbol<*>, Int>()
        val deferredLambdas = HashSet<FirAnonymousFunction>()
        function.body?.accept(object : FirVisitorVoid() {
            private var deferredDepth = 0

            override fun visitElement(element: FirElement) {
                if (element is FirFunction && element !is FirNamedFunction && element !is FirAnonymousFunction) return
                if (element is FirProperty && element.isLocal) locals[element.symbol] = element
                if (element is FirFunctionCall) {
                    val deferredCall = element.calleeReference.toResolvedCallableSymbol()?.callableId?.callableName?.asString() in deferredLambdaCalls
                    element.resolvedArgumentMapping?.forEach { (argument, _) ->
                        val namedOnArgument = writtenArgumentNameThroughWrappers(argument)?.replace("`", "")?.let { name ->
                            name.length > 2 && name.startsWith("on") && name[2] in 'A'..'Z'
                        } == true
                        if (deferredCall || namedOnArgument) {
                            deferredLambdas.addAll(lambdaLiteralsInArgument(argument))
                        }
                    }
                }
                if (element is FirQualifiedAccessExpression && deferredDepth == 0) {
                    element.calleeReference.toResolvedCallableSymbol()?.let { symbol ->
                        val offset = element.source?.startOffset ?: Int.MAX_VALUE
                        reads[symbol] = minOf(reads[symbol] ?: Int.MAX_VALUE, offset)
                    }
                }
                val deferred = element is FirAnonymousFunction && element in deferredLambdas
                if (deferred) deferredDepth++
                element.acceptChildren(this)
                if (deferred) deferredDepth--
            }
        })
        return BodyEvidence(locals, reads)
    }

    private fun lambdaLiteralsInArgument(argument: FirExpression): List<FirAnonymousFunction> {
        val lambdas = ArrayList<FirAnonymousFunction>()
        unwrapArgument(argument).accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirFunctionCall && element.source?.let(::selectorCallExpression) != null) return
                if (element is FirAnonymousFunction && element.isLambda) {
                    lambdas.add(element)
                    return
                }
                element.acceptChildren(this)
            }
        })
        return lambdas
    }
}
