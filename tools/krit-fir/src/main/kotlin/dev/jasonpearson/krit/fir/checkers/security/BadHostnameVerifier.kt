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
import org.jetbrains.kotlin.descriptors.ClassKind
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
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
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
        if (source.kind !is KtRealSourceElementKind) return
        if (declaration.receiverParameter != null || declaration.contextParameters.isNotEmpty()) return
        val params = declaration.valueParameters
        if (params.size != 2) return
        val session = context.session
        if (!hasType(params[0], StandardClassIds.String, session) || !hasType(params[1], sslSession, session)) return
        if (declaration.body == null) return
        // The declaring class must be the nearest container: a local function
        // named verify, even inside a verifier, is not the verifier's verify.
        val owner = context.containingDeclarations.lastOrNull { it != declaration.symbol } as? FirClassSymbol<*> ?: return
        if (!isHostnameVerifier(owner, session)) return
        if (!alwaysReturnsTrue(declaration, requireEveryReturn = (owner as? FirRegularClassSymbol)?.classKind == ClassKind.OBJECT)) return
        report(firstLine(source), MESSAGE)
    }

    /** `HostnameVerifier { _, _ -> true }`: a SAM constructor call whose lambda always returns true. */
    private object SamLambda : FirFunctionCallChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(expression: FirFunctionCall) {
            val source = expression.source ?: return
            if (source.kind !is KtRealSourceElementKind) return
            reportSamConversions(expression)
            val created = expression.resolvedType.fullyExpandedType().lowerBoundIfFlexible()
            if (created.classId != hostnameVerifier) return
            if (expression.calleeReference.toResolvedCallableSymbol()?.origin != FirDeclarationOrigin.SamConstructor) return
            val lambda = (expression.argumentList.arguments.singleOrNull() as? FirAnonymousFunctionExpression)
                ?.anonymousFunction ?: return
            if (lambda.body == null) return
            if (!alwaysReturnsTrue(lambda, requireEveryReturn = true)) return
            report(source, MESSAGE)
        }

        // `setDefaultHostnameVerifier { _, _ -> true }`: a lambda argument the compiler converts to a HostnameVerifier.
        context(context: CheckerContext, reporter: DiagnosticReporter)
        private fun reportSamConversions(call: FirFunctionCall) {
            for (argument in call.argumentList.arguments) {
                val conversion = unwrapArgument(argument) as? FirFunctionTypeConversionExpression ?: continue
                if (conversion.kind !is FirFunctionConversionKind.Sam) continue
                if (conversion.resolvedType.fullyExpandedType().lowerBoundIfFlexible().classId != hostnameVerifier) continue
                val lambda = (conversion.expression as? FirAnonymousFunctionExpression)?.anonymousFunction ?: continue
                if (lambda.body == null) continue
                if (!alwaysReturnsTrue(lambda, requireEveryReturn = true)) continue
                report(lambda.source ?: continue, MESSAGE)
            }
        }
    }

    private fun hasType(param: FirValueParameter, classId: ClassId, session: FirSession): Boolean =
        param.returnTypeRef.coneType.fullyExpandedType(session).lowerBoundIfFlexible().classId == classId

    // Supertypes come from the class's own lookup tags, which are bound to
    // local and anonymous classes, so no class id is resolved from a symbol
    // that may be local.
    private fun isHostnameVerifier(owner: FirClassSymbol<*>, session: FirSession): Boolean =
        inheritsFrom(owner, session, lookupInterfaces = true) { it == hostnameVerifier }

    private fun alwaysReturnsTrue(function: FirFunction, requireEveryReturn: Boolean = false): Boolean {
        var lastReturnTrue = false
        var allReturnsTrue = true
        function.body?.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (element is FirClass) return
                element.acceptChildren(this)
                if (element is FirReturnExpression && element.target.labeledElement === function) {
                    val returned = element.result as? FirLiteralExpression
                    lastReturnTrue = returned?.kind == ConstantValueKind.Boolean && returned.value == true
                    allReturnsTrue = allReturnsTrue && lastReturnTrue
                }
            }
        })
        return lastReturnTrue && (!requireEveryReturn || allReturnsTrue)
    }

    private fun firstLine(source: KtSourceElement): KtSourceElement =
        firstLineAnchor(source, KtTokens.FUN_KEYWORD) ?: source
}
