package dev.jasonpearson.krit.fir.checkers.releaseengineering

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.insideLoggingSink
import dev.jasonpearson.krit.fir.support.isMainFunction
import dev.jasonpearson.krit.fir.support.isPrintToolingPath
import dev.jasonpearson.krit.fir.support.nullableClassType
import dev.jasonpearson.krit.fir.support.unwrapArgument
import dev.jasonpearson.krit.fir.support.unwrapSmartCasts
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirReceiverParameterSymbol
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Flags console output in production code: a call to `kotlin.io.println` /
 * `kotlin.io.print`, or `print` / `println` on `System.out` / `System.err`.
 *
 * Mirrors the Go rule's scope and exemptions:
 * - the call is either unqualified (`println(...)`) or has the receiver
 *   `System.out` / `System.err`; any other explicit receiver is left alone;
 * - an unqualified call counts when it resolves to the kotlin.io built-in, or
 *   to `print` / `println` of `System.out` / `System.err` reached through the
 *   receiver of a lambda (`with(System.out) { println(x) }`,
 *   `System.err.run { print(x) }`), which Go reports as a bare call;
 * - files krit classifies as test files, Kotlin scripts, and files under a
 *   `samples`, `sample`, `demos` or `demo` directory are skipped;
 * - a call whose nearest enclosing named function is annotated `@TaskAction`
 *   (any package, as Go matches the simple name) or is a top-level `main` is
 *   skipped, as is a `@JvmStatic` `main` entry point of an object or
 *   companion object. Lambdas and local classes do not end that search,
 *   local functions do, exactly as Go's nearest `function_declaration`
 *   ancestor.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: Go treats an unqualified `println` as the built-in unless the
 *   file itself declares a function, or imports a symbol, named `println`. A
 *   same-package function in another file, a star-imported one, an inherited
 *   member (also through `with(receiver)`), or an invoked property or local
 *   val named `println` is not console output, so it is not reported. Go
 *   matches `System.out` by spelling, so a local `System` object with an
 *   `out` member is reported by Go and not here. Go matches `@TaskAction` by
 *   spelling; a typealias of the annotation is still a task action here.
 *   Go reports every bare `print` / `println` on an implicit receiver; one
 *   whose receiver is not System.out / System.err (a `PrintWriter` over a
 *   `StringWriter` in `apply`, a `PrintStream` subclass calling its own
 *   member) writes to that stream, not the console, so it is not reported.
 * - Recall: Go stops reporting every unqualified `println` in a file that
 *   declares or imports any `println` (an unrelated member, an overload or an
 *   imported function the argument does not fit, `kotlin.io.print` imported
 *   as `println`), and it misses the package-qualified
 *   `kotlin.io.println(...)`, `java.lang.System.out` (also statically
 *   imported as `out`), an import alias of the built-in or of `System`, a
 *   typealias of `System`, and a parenthesized `(System.out)` receiver. Each
 *   of those resolves to console output and is reported.
 * - Paths: the directory markers are matched on the scan's own spelling of
 *   the path, the string the Go rule tests (see [isNonProductionPath]); a
 *   leading directory counts as Go's tooling markers do.
 */
internal object PrintlnInProduction : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "PrintlnInProduction"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(PrintlnInProduction)
    }

    private const val MESSAGE = "println/print in production code; use a logging framework instead."
    private val KOTLIN_IO = FqName("kotlin.io")
    private val PRINT_NAMES = setOf(Name.identifier("println"), Name.identifier("print"))
    private val SYSTEM = classIdOf("java.lang", "System")
    private val SYSTEM_STREAMS = setOf(
        CallableId(SYSTEM, Name.identifier("out")),
        CallableId(SYSTEM, Name.identifier("err")),
    )
    private val PRINT_STREAMS = listOf(
        classIdOf("java.io", "PrintStream"),
        classIdOf("java.io", "PrintWriter"),
    ).map { it.nullableClassType() }
    private val MAIN = Name.identifier("main")
    private val TASK_ACTION = Name.identifier("TaskAction")
    private val NON_PRODUCTION_DIRECTORIES = listOf("/samples/", "/sample/", "/demos/", "/demo/")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.name !in PRINT_NAMES) return
        if (!isConsolePrint(expression, callee.callableId)) return
        if (isInTestFile()) return
        if (isNonProductionPath(containingScanPath()) || isPrintToolingPath(containingScanPath())) return
        if (insideLoggingSink()) return
        if (isExemptEnclosingFunction()) return
        // Go anchors on the callee name, so a multi-line chain reports on its `.println` line.
        report(expression.calleeReference.source ?: expression.source, MESSAGE)
    }

    context(context: CheckerContext)
    private fun isConsolePrint(expression: FirFunctionCall, callableId: CallableId?): Boolean {
        // kotlin.io.println, however it is spelled: bare, package-qualified, or
        // through an import alias.
        if (callableId != null && callableId.classId == null && callableId.packageName == KOTLIN_IO) return true
        val receiver = expression.explicitReceiver
        if (receiver != null) return isSystemStream(receiver)
        // A bare call Go reports as the built-in that resolves to a JDK print
        // stream through an implicit receiver.
        val dispatch = expression.dispatchReceiver ?: return false
        val dispatchType = runCatching { dispatch.resolvedType }.getOrNull() ?: return false
        if (PRINT_STREAMS.none { dispatchType.isSubtypeOf(it, context.session) }) return false
        return isLambdaReceiverOfSystemStream(dispatch)
    }

    // Whether [receiver] is the receiver of a lambda passed to a call on, or
    // with an argument of, System.out / System.err: `with(System.out) { }`,
    // `System.err.apply { }`.
    context(context: CheckerContext)
    private fun isLambdaReceiverOfSystemStream(receiver: FirExpression): Boolean {
        val thisReceiver = unwrapSmartCasts(receiver) as? FirThisReceiverExpression ?: return false
        val parameter = thisReceiver.calleeReference.boundSymbol as? FirReceiverParameterSymbol ?: return false
        val lambda = parameter.containingDeclarationSymbol as? FirAnonymousFunctionSymbol ?: return false
        val call = context.callsOrAssignments.asReversed().firstNotNullOfOrNull { statement ->
            (statement as? FirFunctionCall)?.takeIf { candidate -> candidate.argumentList.arguments.any { it.passesLambda(lambda) } }
        } ?: return false
        call.explicitReceiver?.let { if (isSystemStream(it)) return true }
        return call.argumentList.arguments.any { !it.passesLambda(lambda) && isSystemStream(it) }
    }

    private fun FirExpression.passesLambda(lambda: FirAnonymousFunctionSymbol): Boolean {
        val value = unwrapArgument(this)
        return value is FirAnonymousFunctionExpression && value.anonymousFunction.symbol == lambda
    }

    private fun isSystemStream(receiver: FirExpression): Boolean {
        val value = unwrapArgument(receiver)
        val access = unwrapSmartCasts(value) as? FirPropertyAccessExpression ?: return false
        val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return false
        return symbol.callableId in SYSTEM_STREAMS
    }

    // Go's nearest `function_declaration` ancestor: the innermost named
    // function, looking through lambdas, anonymous functions, accessors and
    // local classes.
    context(context: CheckerContext)
    private fun isExemptEnclosingFunction(): Boolean {
        val declarations = context.containingDeclarations
        val index = declarations.indexOfLast { it is FirNamedFunctionSymbol }
        val function = declarations.getOrNull(index) as? FirNamedFunctionSymbol ?: return false
        if (isMainFunction(function, declarations.getOrNull(index - 1), context.session)) return true
        if (function.resolvedAnnotationsWithClassIds.any { it.toAnnotationClassId(context.session)?.shortClassName == TASK_ACTION }) {
            return true
        }
        if (function.name != MAIN) return false
        val id = function.callableId ?: return false
        return id.classId == null && !id.isLocal
    }

    /**
     * Go skips test files, Gradle build scripts, `.kts` scripts, and paths
     * containing a `samples`, `sample`, `demos` or `demo` directory
     * (lowercased). [path] is the file's scan path ([FirRule.scanPath]): the
     * exact string the Go rule tests, usually relative to the directory krit
     * ran in, so `samples/proj/src/X.kt` from `krit .` names a top-level
     * samples directory. Separators are not normalized, as in Go.
     */
    internal fun isNonProductionPath(path: String?): Boolean {
        if (path.isNullOrEmpty()) return false
        val base = path.substringAfterLast('/').substringAfterLast('\\')
        if (base == "build.gradle" || base.endsWith(".kts")) return true
        val lower = "/" + path.lowercase().trimStart('/')
        return NON_PRODUCTION_DIRECTORIES.any { it in lower }
    }
}
