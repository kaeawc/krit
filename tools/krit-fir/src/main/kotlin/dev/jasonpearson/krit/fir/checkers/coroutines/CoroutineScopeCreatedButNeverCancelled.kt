package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.bodyInCheckedFile
import dev.jasonpearson.krit.fir.checkers.initializerInCheckedFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.identifierText
import dev.jasonpearson.krit.fir.support.isNonNothingSubtypeOf
import dev.jasonpearson.krit.fir.support.isNullLiteral
import dev.jasonpearson.krit.fir.support.nestedClassIdOf
import dev.jasonpearson.krit.fir.support.nullableClassType
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirGetClassCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirVarargArgumentsExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

internal object CoroutineScopeCreatedButNeverCancelled : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "CoroutineScopeCreatedButNeverCancelled"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(CoroutineScopeCreatedButNeverCancelled)
    }

    private val scopeType = classIdOf("kotlinx.coroutines", "CoroutineScope").nullableClassType()
    private val jobType = classIdOf("kotlinx.coroutines", "Job").nullableClassType()
    private val contextType = classIdOf("kotlin.coroutines", "CoroutineContext").nullableClassType()
    private val elementType = nestedClassIdOf("kotlin.coroutines", "CoroutineContext.Element").nullableClassType()
    private val dispatcherType = classIdOf("kotlinx.coroutines", "CoroutineDispatcher").nullableClassType()
    private val factories = setOf(coroutineId("CoroutineScope"), coroutineId("MainScope"))
    private val jobs = setOf(coroutineId("Job"), coroutineId("SupervisorJob"))
    private val cancellation = setOf("cancel", "cancelChildren", "cancelAndJoin")
    private val fakePrefixes = listOf("Fake", "Test", "Stub", "Dummy", "Noop", "NoOp")
    private val hooks = setOf(
        "close", "dispose", "onCleared", "onDestroy", "release", "onForgotten", "onAbandoned", "shutdown",
        "destroy", "onDispose", "detach", "onDetach", "cleanup", "tearDown",
    )
    private val resets = setOf("reset", "clear", "cancelAll", "cancelChildren", "shutdown", "dispose", "close")

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        val source = declaration.source ?: return
        val initializer = declaration.initializer as? FirFunctionCall ?: return
        if (initializer.calleeReference.toResolvedCallableSymbol()?.callableId !in factories) return
        if (!initializer.resolvedType.isNonNothingSubtypeOf(scopeType)) return
        val owner = context.containingElements.filterIsInstance<FirRegularClass>().lastOrNull {
            it.classKind != ClassKind.OBJECT && it.classKind != ClassKind.ENUM_ENTRY
        } ?: return
        val path = context.containingFile?.path
        if (isTestFile(path) || "/test/" in scanPath(path).orEmpty().replace('\\', '/')) return
        if (fakePrefixes.any { owner.name.asString().startsWith(it) }) return
        val annotations = owner.symbol.resolvedAnnotationClassIds.map { it.shortClassName.asString() }
        if ("Singleton" in annotations) return
        var hasHook = false
        owner.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirNamedFunction && element.name.asString() in hooks) hasHook = true
                element.acceptChildren(this)
            }
        })
        if (!hasHook && annotations.any { it.startsWith("Contributes") || it == "SingleIn" }) return
        if (owner.annotations.any {
                it.annotationTypeRef.coneType.classId?.shortClassName?.asString() == "SingleIn" &&
                    it.argumentMapping.mapping.values.any { argument -> appScopeArgument(argument) }
            }) return
        if (initializer.argumentList.arguments.any { hasParentContext(unwrapArgument(it), 0) }) return
        if (managedInFile(declaration)) return
        val name = identifierText(source) ?: declaration.name.asString()
        report(
            identifierAnchor(source) ?: source,
            "CoroutineScope '$name' is created but never cancelled. Coroutines launched in it can outlive the owner; cancel it from the owner's teardown.",
        )
    }

    context(context: CheckerContext)
    private fun appScopeArgument(expression: FirExpression): Boolean =
        expression is FirGetClassCall &&
            (expression.resolvedType as? ConeClassLikeType)?.typeArguments?.singleOrNull()
                ?.type?.fullyExpandedType()?.classId?.shortClassName?.asString() == "AppScope"

    context(context: CheckerContext)
    private fun hasParentContext(expression: FirExpression, depth: Int): Boolean {
        if (depth > 16) return true
        val current = unwrapSmartCasts(expression, throughNotNull = true)
        if (isNullLiteral(current)) return false
        if (current.resolvedType.isNonNothingSubtypeOf(dispatcherType)) return false
        if (current.resolvedType.classId == classIdOf("kotlin.coroutines", "EmptyCoroutineContext")) return false
        if (current is FirFunctionCall) {
            val id = current.calleeReference.toResolvedCallableSymbol()?.callableId ?: return true
            if (id in jobs) return current.argumentList.arguments.any { !isNullLiteral(unwrapArgument(it)) }
            if (id.classId == classIdOf("kotlin.coroutines", "CoroutineContext") && id.callableName.asString() == "plus") {
                return receiverOf(current)?.let { hasParentContext(it, depth + 1) } != false ||
                    current.argumentList.arguments.any { hasParentContext(unwrapArgument(it), depth + 1) }
            }
        }
        if (current.resolvedType.isNonNothingSubtypeOf(jobType)) return true
        if (current.resolvedType.isNonNothingSubtypeOf(elementType)) return false
        return current.resolvedType.isNonNothingSubtypeOf(contextType)
    }

    @OptIn(SymbolInternals::class)
    context(context: CheckerContext)
    private fun managedInFile(declaration: FirProperty): Boolean {
        val file = context.containingFileSymbol?.fir ?: return true
        var managed = false
        var collected = false
        var cancelled = false
        var functionName: String? = null
        val bindings = mutableMapOf<FirBasedSymbol<*>, FirExpression>()
        file.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (managed) return
                val previous = functionName
                if (element is FirNamedFunction) functionName = element.name.asString()
                when (element) {
                    is FirFunctionCall -> {
                        val symbol = element.calleeReference.toResolvedCallableSymbol()
                        val id = symbol?.unwrapFakeOverrides()?.callableId
                        val receiver = receiverOf(element)
                        if (id?.packageName?.asString() == "kotlin" && id.classId == null &&
                            id.callableName.asString() in setOf("let", "also", "run", "apply") && receiver != null
                        ) {
                            element.argumentList.arguments.map(::unwrapArgument)
                                .filterIsInstance<FirAnonymousFunctionExpression>().forEach { lambda ->
                                    lambda.anonymousFunction.valueParameters.forEach { bindings[it.symbol] = receiver }
                                    lambda.anonymousFunction.receiverParameter?.let { bindings[it.symbol] = receiver }
                                }
                        }
                        val related = receiver?.let { relatesTo(it, declaration.symbol, 0, bindings) }
                        if (isCancellation(element)) {
                            cancelled = true
                            if (related != false) managed = true
                        }
                        if (id?.callableName?.asString() in resets && functionName in hooks && related == true &&
                            couldManage(symbol as? FirFunctionSymbol<*>)
                        ) managed = true
                        if (id != null && !borrowsOnly(id) && element !== declaration.initializer &&
                            element.argumentList.arguments.any { relatesTo(unwrapArgument(it), declaration.symbol, 0, bindings) == true }
                        ) managed = true
                        if (id != null && id.packageName.asString() == "kotlin.collections" && borrowsOnly(id) &&
                            element.argumentList.arguments.any { relatesTo(unwrapArgument(it), declaration.symbol, 0, bindings) == true }
                        ) collected = true
                        if (id?.callableName?.asString() in setOf("cancel", "cancelChildren") &&
                            !isCancellation(element) && related == true &&
                            (id?.callableName?.asString() == "cancelChildren" || couldManage(symbol as? FirFunctionSymbol<*>))
                        ) managed = true
                    }
                    is FirReturnExpression -> if (relatesTo(element.result, declaration.symbol, 0, bindings) == true) managed = true
                    is FirVariableAssignment -> if (relatesTo(element.rValue, declaration.symbol, 0, bindings) == true) managed = true
                    else -> Unit
                }
                if (!managed) element.acceptChildren(this)
                functionName = previous
            }
        })
        return managed || (collected && cancelled)
    }

    context(context: CheckerContext)
    private fun couldManage(symbol: FirFunctionSymbol<*>?): Boolean {
        val body = symbol?.bodyInCheckedFile() ?: return true
        var effect = false
        body.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirFunctionCall) {
                    val id = element.calleeReference.toResolvedCallableSymbol()?.callableId
                    if (id == null || !borrowsOnly(id)) effect = true
                }
                if (element is FirVariableAssignment) effect = true
                if (!effect) element.acceptChildren(this)
            }
        })
        return effect
    }

    context(context: CheckerContext)
    private fun relatesTo(
        expression: FirExpression,
        target: FirPropertySymbol,
        depth: Int,
        bindings: Map<FirBasedSymbol<*>, FirExpression>,
    ): Boolean? {
        if (depth > 16) return null
        val current = unwrapSmartCasts(unwrapArgument(expression), throughNotNull = true)
        when (current) {
            is FirSafeCallExpression -> return relatesTo(current.receiver, target, depth + 1, bindings)
            is FirVarargArgumentsExpression -> {
                val relations = current.arguments.map { relatesTo(it, target, depth + 1, bindings) }
                return if (relations.any { it == true }) true else if (relations.any { it == null }) null else false
            }
            is FirThisReceiverExpression -> {
                val symbol = current.calleeReference.boundSymbol as? FirBasedSymbol<*> ?: return null
                return bindings[symbol]?.let { relatesTo(it, target, depth + 1, bindings) }
            }
            is FirPropertyAccessExpression -> {
                val symbol = current.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides() ?: return null
                if (symbol == target) return true
                bindings[symbol]?.let { return relatesTo(it, target, depth + 1, bindings) }
                val id = symbol.callableId
                if (id != null &&
                    ((id.classId == classIdOf("kotlinx.coroutines", "CoroutineScope") &&
                        id.callableName.asString() == "coroutineContext") || id == coroutineId("job"))
                ) return receiverOf(current)?.let { relatesTo(it, target, depth + 1, bindings) }
                if (symbol is FirPropertySymbol && symbol.isVal && !symbol.hasDelegate && symbol.getterSymbol?.isDefault != false) {
                    symbol.initializerInCheckedFile()?.let { return relatesTo(it, target, depth + 1, bindings) }
                }
                return if (symbol is FirPropertySymbol && resourceType(current)) null else false
            }
            is FirFunctionCall -> {
                val id = current.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides()?.callableId ?: return null
                if (id in factories || id in jobs) return false
                if (id.classId == classIdOf("kotlin.coroutines", "CoroutineContext") && id.callableName.asString() == "get") {
                    return receiverOf(current)?.let { relatesTo(it, target, depth + 1, bindings) }
                }
                if (current.argumentList.arguments.any { relatesTo(unwrapArgument(it), target, depth + 1, bindings) == true }) return true
                return null
            }
            else -> return if (resourceType(current)) null else false
        }
    }

    context(context: CheckerContext)
    private fun resourceType(expression: FirExpression): Boolean =
        expression.resolvedType.isNonNothingSubtypeOf(scopeType) ||
            expression.resolvedType.isNonNothingSubtypeOf(contextType) ||
            expression.resolvedType.isNonNothingSubtypeOf(jobType)

    context(context: CheckerContext)
    private fun isCancellation(call: FirFunctionCall): Boolean {
        val id = call.calleeReference.toResolvedCallableSymbol()?.unwrapFakeOverrides()?.callableId ?: return false
        if (id.callableName.asString() !in cancellation) return false
        val receiver = receiverOf(call) ?: return false
        if (id.packageName.asString() == "kotlinx.coroutines" && id.classId == null) {
            return receiver.resolvedType.isNonNothingSubtypeOf(scopeType) ||
                receiver.resolvedType.isNonNothingSubtypeOf(contextType) || receiver.resolvedType.isNonNothingSubtypeOf(jobType)
        }
        return id.classId == classIdOf("kotlinx.coroutines", "Job")
    }

    private fun borrowsOnly(id: CallableId): Boolean =
        (id.packageName.asString() == "kotlinx.coroutines.flow" && id.classId == null &&
            id.callableName.asString() in setOf("stateIn", "shareIn", "launchIn", "produceIn")) ||
            (id.packageName.asString() == "kotlin.collections" &&
                id.callableName.asString() in setOf("add", "plus", "listOf", "mutableListOf")) ||
            (id.packageName.asString() == "kotlin.io" && id.classId == null && id.callableName.asString() == "println")

    private fun receiverOf(expression: FirQualifiedAccessExpression): FirExpression? =
        expression.explicitReceiver ?: expression.extensionReceiver ?: expression.dispatchReceiver

    private fun coroutineId(name: String) = CallableId(FqName("kotlinx.coroutines"), Name.identifier(name))
}
