package dev.jasonpearson.krit.fir.checkers.potentialbugs

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a call to `kotlin.system.exitProcess` or `java.lang.System.exit`
// whose nearest enclosing named function is not called `main`.
//
// Mirrors the Go ExitOutsideMain rule's Kotlin path:
// - the "inside main" test walks to the nearest enclosing named function (Go's
//   nearest `function_declaration`) and exempts the call only when that
//   function is named `main`, whatever its owner, receiver or parameters.
//   Lambdas, anonymous functions, classes, objects, initializers, accessors
//   and constructors are passed through, as Go passes through their nodes, so
//   `thread { exitProcess(1) }` inside `main` is exempt while a local or
//   member function declared inside `main` is not;
// - a call with no enclosing named function (a property initializer, an
//   `init` block, a getter) is reported;
// - the finding is reported on the call, so on the line where the call
//   expression (including its `System.` / package qualifier) starts.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: the call must resolve to kotlin.system.exitProcess or
//   java.lang.System.exit. Go matches any call named `exitProcess` (a local
//   function, a member, a function-typed value) and any `exit` whose receiver
//   is spelled `System` or `java.lang.System` (a local `object System`).
// - Recall: an import alias of either function, a static import of
//   System.exit, or an import alias of java.lang.System still calls the real
//   function; Go matches only the literal spellings and misses them.
// - A backticked `` fun `main`() `` is the main function; Go compares the raw
//   identifier text including the backticks and reports calls inside it.
internal object ExitOutsideMain : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ExitOutsideMain"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ExitOutsideMain)
    }

    private const val MESSAGE = "Do not call exitProcess() or System.exit() outside of the main function."

    private val exitProcess = CallableId(FqName("kotlin.system"), Name.identifier("exitProcess"))
    private val systemExit = CallableId(ClassId(FqName("java.lang"), Name.identifier("System")), Name.identifier("exit"))
    private val main = Name.identifier("main")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        if (callableId != exitProcess && callableId != systemExit) return

        val enclosingFunction = context.containingDeclarations
            .filterIsInstance<FirNamedFunctionSymbol>()
            .lastOrNull()
        if (enclosingFunction?.name == main) return

        report(expression.source, MESSAGE)
    }
}
