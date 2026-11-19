package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.isMainFunction
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a call to `kotlin.system.exitProcess`, `java.lang.System.exit`,
// `java.lang.Runtime.exit` or `java.lang.Runtime.halt` whose nearest enclosing
// function is not detekt's main function.
//
// Mirrors detekt 1.23.8 and the Go ExitOutsideMain rule's Kotlin path:
// - the "inside main" test walks to the nearest enclosing named or anonymous
//   function (detekt's KtNamedFunction, which `fun(x: Int) { ... }` also is)
//   and exempts the call only when that function is a main function in
//   detekt's sense: named `main`, public and not an override, taking no
//   parameter, `Array<String>` or `vararg String`, and declared at the top
//   level or as a `@JvmStatic` member of an object. Lambdas, classes,
//   initializers, accessors and constructors are passed through, so
//   `thread { exitProcess(1) }` inside `main` is exempt while an anonymous,
//   local or member function declared inside `main` is not;
// - a call with no enclosing function (a property initializer, an `init`
//   block, a getter) is reported;
// - the finding is reported on the callee name, like detekt's
//   KtCallExpression, so a `System` qualifier on the line above does not move
//   it.
//
// Deliberate differences from detekt, pinned by goldens: the parameter type
// is resolved, so `Array<kotlin.String>` or a type alias of `Array<String>`
// is a main parameter (detekt compares the written text), and an extension
// `fun String.main()` is not main (detekt does not look at the receiver).
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: the call must resolve to one of the functions above. Go
//   matches any call named `exitProcess` (a local function, a member, a
//   function-typed value) and any `exit` whose receiver is spelled `System`
//   or `java.lang.System`, or `Runtime.getRuntime()` for `exit` and `halt`
//   (a local `object System`).
// - Recall: an import alias of either function, a static import of
//   System.exit, an import alias of java.lang.System, or a Runtime held in a
//   variable still calls the real function; Go matches only the literal
//   spellings and misses them.
// - The parameter type is resolved (see above); Go compares the written text
//   like detekt.
internal object ExitOutsideMain : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ExitOutsideMain"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ExitOutsideMain)
    }

    private const val MESSAGE = "Do not call exitProcess() or System.exit() outside of the main function."

    private val exitProcess = CallableId(FqName("kotlin.system"), Name.identifier("exitProcess"))
    private val systemExit = CallableId(classIdOf("java.lang", "System"), Name.identifier("exit"))
    private val runtimeClass = classIdOf("java.lang", "Runtime")
    private val runtimeExit = CallableId(runtimeClass, Name.identifier("exit"))
    private val runtimeHalt = CallableId(runtimeClass, Name.identifier("halt"))
    private val exits = setOf(exitProcess, systemExit, runtimeExit, runtimeHalt)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        if (callableId !in exits) return

        val declarations = context.containingDeclarations
        val enclosing = declarations.indexOfLast { it is FirNamedFunctionSymbol || it is FirAnonymousFunctionSymbol && !it.isLambda }
        val function = declarations.getOrNull(enclosing) as? FirNamedFunctionSymbol
        if (function != null && isMainFunction(function, declarations.getOrNull(enclosing - 1), context.session)) return

        report(expression.calleeReference.source ?: expression.source, MESSAGE)
    }
}
