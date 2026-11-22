package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

internal object RunBlockingInTest : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "RunBlockingInTest"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(RunBlockingInTest)
    }

    private val blocking = CallableId(FqName("kotlinx.coroutines"), Name.identifier("runBlocking"))
    private val testAnnotations = setOf(
        classIdOf("org.junit", "Test"),
        classIdOf("org.junit.jupiter.api", "Test"),
        classIdOf("kotlin.test", "Test"),
        classIdOf("org.testng.annotations", "Test"),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind ||
            expression.calleeReference.toResolvedCallableSymbol()?.callableId != blocking
        ) return
        val containingElements = context.containingElements
        val testFunction = containingElements.filterIsInstance<FirNamedFunction>().lastOrNull()
            ?.takeIf { isTestFunction(it) } ?: return
        val syntax = RunBlockingInTestSyntax.create(source) ?: return
        if (syntax.name(syntax.call) != "runBlocking" || !syntax.goTestAnnotationSpelling()) return
        val path = containingScanPath() ?: return
        val markers = configuredMarkers()
        val containingClasses = containingElements.takeWhile { it !== testFunction }.filterIsInstance<FirRegularClass>()
        if (RunBlockingInTestExemptions.exempt(syntax, path, markers, containingClasses)) return
        val message = if (syntax.insideRunTest()) {
            "Remove the nested `runBlocking`; the test already runs its body in `runTest`."
        } else {
            "Use `runTest` instead of `runBlocking` in test functions."
        }
        report(lightSourceOf(syntax.call, source), message)
    }

    context(context: CheckerContext)
    private fun isTestFunction(function: FirNamedFunction): Boolean =
        function.symbol.resolvedAnnotationsWithClassIds.any {
            it.toAnnotationClassId(context.session) in testAnnotations
        }

    private fun configuredMarkers(): List<String> = when (val value = config()["realDispatcherMarkers"]) {
        is String -> listOf(value)
        is List<*> -> value.mapNotNull { item ->
            when (item) {
                is String -> item
                is Map<*, *> -> item["value"]?.toString()
                else -> null
            }
        }
        else -> emptyList()
    }
}
