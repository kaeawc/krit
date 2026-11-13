package dev.jasonpearson.krit.fir.checkers.compose

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.support.hasAnnotationNamed
import dev.jasonpearson.krit.fir.support.simpleName
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid

// Exemptions the keyless-remember and constant-key-effect rules share. Both Go rules call
// composeKeyRuleExemptFunction: preview and sample scaffolding, test-support sources, @VisibleForTesting
// functions, Fake/Test classes, and private helpers that only previews call.

internal val androidxPreviewAnnotations = setOf(
    "PreviewLightDark", "PreviewFontScale", "PreviewScreenSizes", "PreviewDynamicColors",
)

private val scaffoldingNameAffixes = listOf("preview", "sample", "fixture")

// Directories that hold Android test-support code (Go isAndroidTestSupportArtifactSource), matched on the scan path.
private val testSupportMarkers = listOf(
    "-testing/", "/testing/", "/test-utils/", "-test-utils/", "/test-fixtures/", "/testfixtures/", "/fakes/",
    "/fake/", "/mocks/", "/mock/", "/testing-ui/", "/ui-testing/", "/maestro-runner/", "/idling-resources/",
    "/idlingresources/", "/shared-instrumentation/", "/instrumentation-utils/", "/instrumentation-tests/",
    "/instrumentationtests/", "/android-test-utils/", "/androidtest-utils/", "/testutils/", "/testinternal/",
    "/testdebug/", "/testrelease/", "/testwrapper/", "/espressomodule.kt",
)

/** True for a function named or annotated as a preview, sample or fixture. */
internal fun FirNamedFunction.isPreviewOrSample(): Boolean {
    val lower = name.asString().lowercase()
    if (scaffoldingNameAffixes.any { lower.startsWith(it) || lower.endsWith(it) }) return true
    return annotations.any { annotation ->
        val simple = annotation.simpleName().orEmpty()
        simple in androidxPreviewAnnotations || simple.endsWith("Preview")
    }
}

context(context: CheckerContext)
internal fun FirRule.isTestSupportSource(): Boolean {
    if (isInTestFile()) return true
    val path = containingScanPath()?.replace('\\', '/')?.lowercase() ?: return false
    return testSupportMarkers.any { it in path }
}

/**
 * True when [function] is scaffolding that a keyless `remember` or a constant-keyed effect is deliberate in: it
 * sits in a test-support source or inside a preview or sample function, is `@VisibleForTesting`, belongs to a
 * Fake/Test class, or is a private helper that only previews in this file call.
 */
context(context: CheckerContext)
internal fun FirRule.isComposeKeyRuleExempt(function: FirNamedFunction): Boolean {
    if (isTestSupportSource()) return true
    val enclosing = context.containingElements
    if (enclosing.filterIsInstance<FirNamedFunction>().any { it.isPreviewOrSample() }) return true
    if (function.hasAnnotationNamed(setOf("VisibleForTesting"))) return true
    val className = enclosing.filterIsInstance<FirRegularClass>().lastOrNull()?.name?.asString().orEmpty()
    if (className.startsWith("Fake") || className.contains("Test")) return true
    return isPrivateHelperOfPreviews(function)
}

/**
 * True when [function] is private and every call to it in its file sits in a preview or sample function. Go also
 * exempts internal helpers, but those are usually called from other files; only private ones are safe.
 */
context(context: CheckerContext)
internal fun isPrivateHelperOfPreviews(function: FirNamedFunction): Boolean {
    if (function.status.visibility != Visibilities.Private) return false
    val file = context.containingElements.filterIsInstance<FirFile>().firstOrNull() ?: return false
    return onlyPreviewsCall(file, function)
}

private fun onlyPreviewsCall(file: FirFile, function: FirNamedFunction): Boolean {
    var calls = 0
    var fromPreview = 0
    file.accept(object : FirVisitorVoid() {
        private val functions = ArrayDeque<FirNamedFunction>()
        override fun visitElement(element: FirElement) {
            if (element is FirNamedFunction) functions.addLast(element)
            if (element is FirFunctionCall && element.calleeReference.toResolvedCallableSymbol() == function.symbol) {
                calls++
                if (functions.any { it.isPreviewOrSample() }) fromPreview++
            }
            element.acceptChildren(this)
            if (element is FirNamedFunction) functions.removeLast()
        }
    })
    return calls > 0 && calls == fromPreview
}
