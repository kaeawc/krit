package dev.jasonpearson.krit.fir.checkers.testingquality

import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.GoTestAssertionNames
import dev.jasonpearson.krit.fir.support.expandedClassType
import dev.jasonpearson.krit.fir.support.identifierAnchor
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirGetClassCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.types.ConeKotlinTypeProjection
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens

internal object TestWithoutAssertionTestBody {
    context(context: CheckerContext, reporter: DiagnosticReporter)
    fun checkTest(
        function: FirNamedFunction,
        owners: List<FirRegularClass>,
        file: FirFile,
        patterns: List<String>,
    ) {
        val source = function.source
        val body = function.body
        if (source == null || source.kind !is KtRealSourceElementKind || body == null) return
        if (isIgnoredTest(function, owners) || expectsException(function)) return
        if (hasNoTestBody(body) || documentsNoException(function, body)) return
        val evidence = TestWithoutAssertionEvidence(file, context.session, patterns)
        if (evidence.preventsFinding(body, depth = 0, helper = false) ||
            TestWithoutAssertionCalls.trailingCheck(body, depth = 0)) return
        with(TestWithoutAssertion) {
            report(identifierAnchor(source), "Test function has no assertion; add a verification.")
        }
    }

    fun resultOf(element: FirElement?): FirElement? = when (element) {
        is FirReturnExpression -> resultOf(element.result)
        is FirBlock -> resultOf(element.statements.lastOrNull())
        else -> element
    }

    context(context: CheckerContext)
    private fun expectsException(function: FirNamedFunction): Boolean = function.annotations.any { annotation ->
        if (annotation.annotationTypeRef.coneType.classId != classIdOf("org.junit", "Test")) return@any false
        val expected = annotation.argumentMapping.mapping.entries.firstOrNull { it.key.asString() == "expected" }
            ?.value ?: return@any false
        val literal = unwrapArgument(expected) as? FirGetClassCall
        val argument = literal?.resolvedType?.expandedClassType()?.typeArguments?.singleOrNull()
            as? ConeKotlinTypeProjection
        argument?.type?.classId?.asSingleFqName()?.asString() != "org.junit.Test.None"
    }

    private fun isUnitBody(body: FirBlock): Boolean {
        if (body.statements.size != 1) return false
        val result = resultOf(body) as? FirExpression ?: return false
        val source = result.source ?: return false
        return lightText(source, source.lighterASTNode).trim() == "Unit"
    }

    private fun isTodoBody(body: FirBlock): Boolean {
        if (body.statements.size != 1) return false
        val call = resultOf(body) as? FirFunctionCall ?: return false
        val id = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return false
        return id.packageName.asString() == "kotlin" && id.callableName.asString() == "TODO"
    }

    private fun hasNoTestBody(body: FirBlock): Boolean =
        body.statements.isEmpty() || isUnitBody(body) || isTodoBody(body)

    private fun isIgnoredTest(function: FirNamedFunction, owners: List<FirRegularClass>): Boolean =
        TestWithoutAssertionGoSupport.isIgnored(function) || owners.any(TestWithoutAssertionGoSupport::isIgnored)

    internal fun comments(source: KtSourceElement?, before: Int = Int.MAX_VALUE): String {
        if (source == null) return ""
        val texts = ArrayList<String>()
        fun collect(node: com.intellij.lang.LighterASTNode) {
            if (node.tokenType in KtTokens.COMMENTS && node.startOffset < before) texts += lightText(source, node)
            else lightChildren(source, node).forEach(::collect)
        }
        collect(source.lighterASTNode)
        return texts.joinToString(" ").lowercase().replace('’', '\'')
    }

    private fun documentsNoException(function: FirNamedFunction, body: FirBlock): Boolean =
        nameDocumentsNoException(function) || displayNameDocumentsNoException(function) ||
            commentsDocumentNoException(function, body)
}

private fun nameDocumentsNoException(function: FirNamedFunction): Boolean {
    val name = function.name.asString().lowercase().replace('_', ' ').replace('-', ' ')
    val squashed = name.replace(" ", "")
    return listOf("smoke", "noop", "doesnotthrow", "doesnotcrash").any { it in squashed } ||
        GoTestAssertionNames.noExceptionMarkers.any { it in name || it.replace(" ", "") in squashed }
}

private fun displayNameDocumentsNoException(function: FirNamedFunction): Boolean =
    function.annotations.any(::displayNameDocumentsNoException)

private fun displayNameDocumentsNoException(annotation: FirAnnotation): Boolean {
    if (annotation.annotationTypeRef.coneType.classId != classIdOf("org.junit.jupiter.api", "DisplayName")) return false
    val displayName = annotation.argumentMapping.mapping.values.singleOrNull()
        ?.let(::unwrapArgument) as? FirLiteralExpression ?: return false
    val display = (displayName.value as? String)?.lowercase()?.replace('_', ' ')?.replace('-', ' ') ?: return false
    val squashedDisplay = display.replace(" ", "")
    return "smoke" in display || GoTestAssertionNames.noExceptionMarkers.any {
        it in display || it.replace(" ", "") in squashedDisplay
    }
}

private fun commentsDocumentNoException(function: FirNamedFunction, body: FirBlock): Boolean {
    val headerComments = TestWithoutAssertionTestBody.comments(
        function.source,
        body.source?.startOffset ?: Int.MAX_VALUE,
    )
    if (GoTestAssertionNames.noExceptionMarkers.any { it in headerComments }) return true
    val bodyComments = TestWithoutAssertionTestBody.comments(body.source)
    return GoTestAssertionNames.smokePhrases.any { it in bodyComments } ||
        bodyComments.isNotEmpty() && composeSmokeComment(body, bodyComments)
}

private fun composeSmokeComment(body: FirBlock, bodyComments: String): Boolean {
    var setContent = false
    var idle = false
    body.accept(object : FirVisitorVoid() {
        override fun visitElement(element: FirElement) {
            if (element is FirFunctionCall) {
                val id = element.calleeReference.toResolvedCallableSymbol()?.callableId
                if (id != null && isInPackage(id.packageName.asString(), "androidx.compose.ui.test")) {
                    setContent = setContent || id.callableName.asString() == "setContent"
                    idle = idle || id.callableName.asString() == "waitForIdle"
                }
            }
            element.acceptChildren(this)
        }
    })
    return setContent && idle && GoTestAssertionNames.composeSmokeCommentWords.any { it in bodyComments }
}

private fun isInPackage(actual: String, expected: String): Boolean =
    actual == expected || actual.startsWith("$expected.")
