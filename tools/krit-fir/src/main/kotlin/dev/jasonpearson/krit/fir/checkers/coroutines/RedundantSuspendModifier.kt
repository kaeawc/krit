package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.hasSuspendFunctionType
import dev.jasonpearson.krit.fir.support.modifierKeywordAnchor
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirAnonymousObject
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.InlineStatus
import org.jetbrains.kotlin.fir.declarations.utils.isAbstract
import org.jetbrains.kotlin.fir.declarations.utils.isActual
import org.jetbrains.kotlin.fir.declarations.utils.isExpect
import org.jetbrains.kotlin.fir.declarations.utils.isExternal
import org.jetbrains.kotlin.fir.declarations.utils.isOpen
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.declarations.utils.isSuspend
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirImplicitInvokeCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a `suspend` function whose body never suspends: the modifier forces
// every caller into a coroutine for nothing.
//
// Mirrors the Go RedundantSuspendModifier rule, which looks for call
// expressions in the body by name (a fixed list of coroutine functions, any
// project function declared `suspend`, any member call on a non-local
// receiver) and stays silent when any call could suspend. The checker reads
// the compiler's answer instead: the body suspends when it contains a call
// that resolves to a suspend function (an invoked suspend function value and a
// `for` over a channel included, since the compiler desugars the loop to
// calls) or reads `kotlin.coroutines.coroutineContext`. Like Go it skips open,
// abstract, override, actual, expect and external functions and interface
// members, whose signature a caller or a supertype fixes. It also skips a body
// that only calls TODO() or throws: the modifier belongs to a stub whose
// implementation will suspend.
//
// The walk enters a lambda only when the lambda runs in place (passed to an
// inline function and not itself a suspend lambda); the lambda of `launch`,
// `flow` or `async` has its own suspend context, so a suspend call there does
// not justify the outer modifier. Go counts such a call because it reads every
// lambda as part of the function. Local functions, classes and objects own
// their scope.
internal object RedundantSuspendModifier : FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "RedundantSuspendModifier"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val namedFunctionCheckers = setOf(RedundantSuspendModifier)
    }

    private val todo = CallableId(FqName("kotlin"), Name.identifier("TODO"))
    private val coroutineContext = CallableId(FqName("kotlin.coroutines"), Name.identifier("coroutineContext"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind || !declaration.symbol.isSuspend) return
        if (declaration.isSignatureFixed()) return
        if (insideInterface()) return
        val body = declaration.body ?: return
        if (isPlaceholder(body)) return
        val finder = SuspendFinder()
        body.accept(finder)
        if (finder.found) return
        val anchor = modifierKeywordAnchor(source, KtTokens.SUSPEND_KEYWORD) ?: source
        report(anchor, "Function '${declaration.name.asString()}' has a redundant suspend modifier. No suspend calls found inside.")
    }

    // A body that only calls TODO() or throws is a stub awaiting its implementation.
    private fun isPlaceholder(body: FirBlock): Boolean {
        var statement = body.statements.singleOrNull() ?: return false
        if (statement is FirReturnExpression) statement = statement.result
        return statement is FirThrowExpression ||
            (statement as? FirFunctionCall)?.calleeReference?.toResolvedCallableSymbol()?.callableId == todo
    }

    private fun FirNamedFunction.isSignatureFixed(): Boolean =
        isOpen || isAbstract || isOverride || isActual || isExpect || isExternal

    // Go looks at the nearest enclosing class declaration; an object literal is not one.
    context(context: CheckerContext)
    private fun insideInterface(): Boolean {
        val owner = context.containingDeclarations.lastOrNull { it is FirRegularClassSymbol } as? FirRegularClassSymbol
        return owner?.classKind == ClassKind.INTERFACE
    }

    private class SuspendFinder : FirVisitorVoid() {
        var found = false

        override fun visitElement(element: FirElement) {
            if (!found) element.acceptChildren(this)
        }

        override fun visitFunctionCall(functionCall: FirFunctionCall) {
            val callee = functionCall.calleeReference.toResolvedCallableSymbol() as? FirNamedFunctionSymbol
            if (callee?.isSuspend == true) {
                found = true
                return
            }
            visitElement(functionCall)
        }

        override fun visitImplicitInvokeCall(implicitInvokeCall: FirImplicitInvokeCall) {
            visitFunctionCall(implicitInvokeCall)
        }

        override fun visitPropertyAccessExpression(propertyAccessExpression: FirPropertyAccessExpression) {
            if (propertyAccessExpression.calleeReference.toResolvedCallableSymbol()?.callableId == coroutineContext) {
                found = true
                return
            }
            visitElement(propertyAccessExpression)
        }

        override fun visitAnonymousFunction(anonymousFunction: FirAnonymousFunction) {
            val inPlace = anonymousFunction.inlineStatus == InlineStatus.Inline ||
                anonymousFunction.inlineStatus == InlineStatus.CrossInline
            if (inPlace && !hasSuspendFunctionType(anonymousFunction)) visitElement(anonymousFunction)
        }

        override fun visitNamedFunction(namedFunction: FirNamedFunction) {
            // A local function owns its scope: its calls do not suspend the outer one.
        }

        override fun visitRegularClass(regularClass: FirRegularClass) {
            // A local class owns its scope.
        }

        override fun visitAnonymousObject(anonymousObject: FirAnonymousObject) {
            // An object literal owns its scope.
        }
    }
}
