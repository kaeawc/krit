package dev.jasonpearson.krit.fir.checkers.releaseengineering

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.types.constructClassLikeType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.ClassId
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
 *   to `print` / `println` of a JDK `PrintStream` / `PrintWriter` reached
 *   through an implicit receiver (`with(System.out) { println(x) }`), which
 *   Go reports as a bare call;
 * - files krit classifies as test files, Kotlin scripts, and files under a
 *   `samples`, `sample`, `demos` or `demo` directory are skipped;
 * - a call whose nearest enclosing named function is annotated `@TaskAction`
 *   (any package, as Go matches the simple name) or is a top-level `main` is
 *   skipped. Lambdas and local classes do not end that search, local
 *   functions do, exactly as Go's nearest `function_declaration` ancestor.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: Go treats an unqualified `println` as the built-in unless the
 *   file itself declares a function, or imports a symbol, named `println`. A
 *   same-package function in another file, a star-imported one, an inherited
 *   member, or an invoked property named `println` is not console output, so
 *   it is not reported. Go matches `System.out` by spelling, so a local
 *   `System` object with an `out` member is reported by Go and not here.
 *   Go matches `@TaskAction` by spelling; a typealias of the annotation is
 *   still a task action here.
 * - Recall: Go stops reporting every unqualified `println` in a file that
 *   declares an unrelated `println` member, and it misses the
 *   package-qualified `kotlin.io.println(...)`, `java.lang.System.out` (also
 *   statically imported as `out`), an import alias of the built-in or of
 *   `System`, and a parenthesized `(System.out)` receiver. Each of those is
 *   console output and is reported.
 * - Paths: the directory markers are matched below the working directory
 *   (see [isNonProductionPath]).
 */
internal object PrintlnInProduction : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "PrintlnInProduction"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(PrintlnInProduction)
    }

    private const val MESSAGE = "println/print in production code; use a logging framework instead."
    private val KOTLIN_IO = FqName("kotlin.io")
    private val PRINT_NAMES = setOf(Name.identifier("println"), Name.identifier("print"))
    private val SYSTEM = ClassId.fromString("java/lang/System")
    private val SYSTEM_STREAMS = setOf(
        CallableId(SYSTEM, Name.identifier("out")),
        CallableId(SYSTEM, Name.identifier("err")),
    )
    private val PRINT_STREAMS = listOf(
        ClassId.fromString("java/io/PrintStream"),
        ClassId.fromString("java/io/PrintWriter"),
    ).map { it.constructClassLikeType(emptyArray(), isMarkedNullable = true) }
    private val MAIN = Name.identifier("main")
    private val TASK_ACTION = Name.identifier("TaskAction")
    private val NON_PRODUCTION_DIRECTORIES = listOf("/samples/", "/sample/", "/demos/", "/demo/")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        if (callee.name !in PRINT_NAMES) return
        if (!isConsolePrint(expression, callee.callableId)) return
        if (isInTestFile()) return
        if (isNonProductionPath(context.containingFile?.path)) return
        if (isExemptEnclosingFunction()) return
        report(expression.source, MESSAGE)
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
        val dispatchType = expression.dispatchReceiver?.let { runCatching { it.resolvedType }.getOrNull() } ?: return false
        return PRINT_STREAMS.any { dispatchType.isSubtypeOf(it, context.session) }
    }

    private fun isSystemStream(receiver: FirExpression): Boolean {
        val access = unwrapReceiver(receiver) as? FirPropertyAccessExpression ?: return false
        val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return false
        return symbol.callableId in SYSTEM_STREAMS
    }

    private tailrec fun unwrapReceiver(receiver: FirExpression): FirExpression = when (receiver) {
        is FirCheckedSafeCallSubject -> unwrapReceiver(receiver.originalReceiverRef.value)
        is FirSmartCastExpression -> unwrapReceiver(receiver.originalExpression)
        else -> receiver
    }

    // Go's nearest `function_declaration` ancestor: the innermost named
    // function, looking through lambdas, anonymous functions, accessors and
    // local classes.
    context(context: CheckerContext)
    private fun isExemptEnclosingFunction(): Boolean {
        val function = context.containingDeclarations.lastOrNull { it is FirNamedFunctionSymbol } as? FirNamedFunctionSymbol
            ?: return false
        if (function.resolvedAnnotationsWithClassIds.any { it.toAnnotationClassId(context.session)?.shortClassName == TASK_ACTION }) {
            return true
        }
        if (function.name != MAIN) return false
        val id = function.callableId ?: return false
        return id.classId == null && !id.isLocal
    }

    /**
     * Go skips test files, Gradle build scripts, `.kts` scripts, and paths
     * containing `/samples/`, `/sample/`, `/demos/` or `/demo/` (lowercased).
     * Go tests the scan's own spelling of the path, usually relative to the
     * working directory krit (and so krit-fir) runs in; the compiler sees the
     * absolute path. The markers are therefore matched only below the working
     * directory, so a project checked out under a `samples` directory is not
     * skipped wholesale, and a marker directory at the top of the scanned tree
     * still counts.
     */
    internal fun isNonProductionPath(path: String?, workingDirectory: String? = System.getProperty("user.dir")): Boolean {
        if (path == null) return false
        val normalized = path.replace('\\', '/')
        val base = normalized.substringAfterLast('/')
        if (base == "build.gradle" || normalized.endsWith(".kts")) return true
        val root = workingDirectory?.replace('\\', '/')?.trimEnd('/')
        val scoped = if (root != null && normalized.startsWith("$root/")) normalized.substring(root.length) else normalized
        val lower = scoped.lowercase()
        return NON_PRODUCTION_DIRECTORIES.any { it in lower }
    }
}
