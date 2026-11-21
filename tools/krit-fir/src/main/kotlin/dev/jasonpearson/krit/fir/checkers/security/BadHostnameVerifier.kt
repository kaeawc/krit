package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.firstLineAnchor
import dev.jasonpearson.krit.fir.support.inheritsFrom
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.FirDeclarationOrigin
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirFunctionConversionKind
import org.jetbrains.kotlin.fir.expressions.FirFunctionTypeConversionExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.FirTryExpression
import org.jetbrains.kotlin.fir.expressions.FirWhenExpression
import org.jetbrains.kotlin.fir.expressions.FirWrappedExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isNothing
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds
import org.jetbrains.kotlin.types.ConstantValueKind

internal object BadHostnameVerifier :
    FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "BadHostnameVerifier"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val namedFunctionCheckers = setOf(BadHostnameVerifier)
    }
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(SamLambda)
    }

    private val verify = Name.identifier("verify")
    private val sslPackage = FqName("javax.net.ssl")
    private val hostnameVerifier = classIdOf(sslPackage, "HostnameVerifier")
    private val sslSession = classIdOf(sslPackage, "SSLSession")

    private const val MESSAGE =
        "`verify` always returns `true`, which could cause insecure network traffic due to trusting TLS/SSL server certificates for wrong hostnames"

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        if (declaration.name != verify) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind || !hasVerifierSignature(declaration, context)) return
        val session = context.session
        // The declaring class must be the nearest container: a local function
        // named verify, even inside a verifier, is not the verifier's verify.
        val owner = context.containingDeclarations.lastOrNull { it != declaration.symbol }
            as? FirClassSymbol<*>
        if (owner != null && isHostnameVerifier(owner, session) && alwaysReturnsTrue(declaration)) {
            report(firstLine(source), MESSAGE)
        }
    }

    private fun hasVerifierSignature(
        declaration: FirNamedFunction,
        checkerContext: CheckerContext,
    ): Boolean {
        val params = declaration.valueParameters
        val hasSupportedShape = declaration.receiverParameter == null &&
            declaration.contextParameters.isEmpty() && params.size == 2
        val hasExpectedParameters = hasSupportedShape &&
            hasType(params[0], StandardClassIds.String, checkerContext.session) &&
            hasType(params[1], sslSession, checkerContext.session)
        return hasExpectedParameters && declaration.body != null
    }

    /** `HostnameVerifier { _, _ -> true }`: a SAM constructor call whose lambda always returns true. */
    private object SamLambda : FirFunctionCallChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirFunctionCall) {
            val source = expression.source ?: return
            if (source.kind !is KtRealSourceElementKind) return
            reportSamConversions(expression)
            reportSamConstructor(expression, source)
        }

        context(context: CheckerContext, reporter: DiagnosticReporter)
        private fun reportSamConstructor(expression: FirFunctionCall, source: KtSourceElement) {
            val isHostnameVerifierConstructor =
                expression.resolvedType.fullyExpandedType().lowerBoundIfFlexible().classId == hostnameVerifier
            val isSamConstructor = isHostnameVerifierConstructor &&
                expression.calleeReference.toResolvedCallableSymbol()?.origin == FirDeclarationOrigin.SamConstructor
            if (!isHostnameVerifierConstructor || !isSamConstructor) return
            val lambda = (expression.argumentList.arguments.singleOrNull() as? FirAnonymousFunctionExpression)
                ?.anonymousFunction ?: return
            if (lambda.body != null && alwaysReturnsTrue(lambda)) report(source, MESSAGE)
        }

        // `setDefaultHostnameVerifier { _, _ -> true }`: a lambda argument the compiler converts to a HostnameVerifier.
        context(context: CheckerContext, reporter: DiagnosticReporter)
        private fun reportSamConversions(call: FirFunctionCall) {
            call.argumentList.arguments.forEach { reportSamConversion(it) }
        }

        context(context: CheckerContext, reporter: DiagnosticReporter)
        private fun reportSamConversion(argument: FirExpression) {
            val lambda = convertedHostnameVerifierLambda(argument) ?: return
            val source = lambda.source ?: return
            report(source, MESSAGE)
        }

        context(context: CheckerContext, reporter: DiagnosticReporter)
        private fun convertedHostnameVerifierLambda(argument: FirExpression): FirFunction? {
            val conversion = unwrapArgument(argument) as? FirFunctionTypeConversionExpression ?: return null
            val isSamConversion = conversion.kind is FirFunctionConversionKind.Sam
            val targetsHostnameVerifier = isSamConversion &&
                conversion.resolvedType.fullyExpandedType().lowerBoundIfFlexible().classId == hostnameVerifier
            if (!targetsHostnameVerifier) return null
            return (conversion.expression as? FirAnonymousFunctionExpression)?.anonymousFunction
                ?.takeIf { it.body != null && alwaysReturnsTrue(it) }
        }
    }

    private fun hasType(param: FirValueParameter, classId: ClassId, session: FirSession): Boolean =
        param.returnTypeRef.coneType.fullyExpandedType(session).lowerBoundIfFlexible().classId == classId

    // Supertypes come from the class's own lookup tags, which are bound to
    // local and anonymous classes, so no class id is resolved from a symbol
    // that may be local.
    private fun isHostnameVerifier(owner: FirClassSymbol<*>, session: FirSession): Boolean =
        inheritsFrom(owner, session, lookupInterfaces = true) { it == hostnameVerifier }

    private fun alwaysReturnsTrue(function: FirFunction): Boolean {
        var sawReturn = false
        var returnsAreTrue = true
        var hasRejectingPath = false
        function.body?.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirClass) return
                if (isRejectingExpression(element)) hasRejectingPath = true
                element.acceptChildren(this)
                if (element is FirReturnExpression && element.target.labeledElement === function) {
                    sawReturn = true
                    returnsAreTrue = returnsAreTrue && isLiteralTruePath(element.result)
                }
            }
        })
        return sawReturn && returnsAreTrue && !hasRejectingPath
    }

    private fun isRejectingExpression(element: FirElement): Boolean =
        element is FirThrowExpression || element is FirFunctionCall &&
            (declaredReturnTypeIsNothing(element) || isRejectingGuard(element))

    private fun declaredReturnTypeIsNothing(call: FirFunctionCall): Boolean =
        (call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol)
            ?.resolvedReturnType?.isNothing == true

    private fun isLiteralTruePath(expression: FirExpression?): Boolean = when (expression) {
        is FirLiteralExpression -> expression.kind == ConstantValueKind.Boolean && expression.value == true
        is FirWrappedExpression -> isLiteralTruePath(expression.expression)
        is FirWhenExpression -> expression.branches.isNotEmpty() &&
            expression.branches.all { isLiteralTruePath(it.result) }
        is FirTryExpression -> isLiteralTruePath(expression.tryBlock) &&
            expression.catches.all { isLiteralTruePath(it.block) }
        is FirBlock -> isLiteralTruePath(expression.statements.lastOrNull() as? FirExpression)
        else -> false
    }

    private fun isRejectingGuard(call: FirFunctionCall): Boolean {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol ?: return false
        val callableId = callee.callableId
        val name = callableId.callableName.asString()
        return when {
            callableId.packageName == FqName("kotlin") && name in setOf(
                "require", "requireNotNull", "check", "checkNotNull",
            ) -> true
            callableId.packageName == FqName("java.util") &&
                callableId.className?.asString() == "Objects" && name == "requireNonNull" -> true
            else -> false
        }
    }

    private fun firstLine(source: KtSourceElement): KtSourceElement =
        firstLineAnchor(source, KtTokens.FUN_KEYWORD) ?: source
}
