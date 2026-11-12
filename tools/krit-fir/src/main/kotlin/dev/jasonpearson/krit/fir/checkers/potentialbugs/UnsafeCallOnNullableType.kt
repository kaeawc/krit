package dev.jasonpearson.krit.fir.checkers.potentialbugs

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.declaringSourcePath
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.isNullLiteral
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.qualifiedCall
import dev.jasonpearson.krit.fir.support.significantChildren
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.contracts.description.LogicOperationKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirCheckNotNullCallChecker
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirAnonymousFunction
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirBlock
import org.jetbrains.kotlin.fir.expressions.FirBooleanOperatorExpression
import org.jetbrains.kotlin.fir.expressions.FirCheckNotNullCall
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirElvisExpression
import org.jetbrains.kotlin.fir.expressions.FirEqualityOperatorCall
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirJump
import org.jetbrains.kotlin.fir.expressions.FirOperation
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirThrowExpression
import org.jetbrains.kotlin.fir.expressions.argument
import org.jetbrains.kotlin.fir.expressions.arguments
import org.jetbrains.kotlin.fir.expressions.unwrapSmartcastExpression
import org.jetbrains.kotlin.fir.references.FirNamedReference
import org.jetbrains.kotlin.fir.references.toResolvedPropertySymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.references.toResolvedVariableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirVariableSymbol
import org.jetbrains.kotlin.fir.types.canBeNull
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens

/**
 * Flags a not-null assertion (`!!`) on an operand the compiler types as nullable.
 *
 * Mirrors the Go rule, which reports only an operand typed NULLABLE (as detekt does): a platform
 * type and a stable smart cast to non-null are not reported, a type parameter with a nullable bound
 * is. The checker reads that type from the compiler, so the Go flow guards that approximate smart
 * casts (`if (x != null)`, early return, same-block assignment, short-circuit) are not needed. Kept
 * from Go:
 * - test and test-support files (`isTestSupportFile` and the artifact directories), and `@Preview`,
 *   `sample`, `fake`, `mock`, `stub` and `fixture` functions with the `customPreviewWildcard` and
 *   `customPreviewPrefixes` options;
 * - `Type::class.simpleName!!` and a map index operand (`map[key]!!`, owned by
 *   MapGetWithNotNullAssertionOperator);
 * - the guards the compiler cannot see for an unstable property: an earlier `x ?: return`,
 *   `check(x != null)`, `require(...)`, `assert(...)`, `requireNotNull(x)` or `checkNotNull(x)` in an
 *   enclosing block of the same function or lambda, and `.filter { it.f != null }.map { it.f!! }`;
 * - the `requireX(): T = field!!` expression-body exemption;
 * - Go's message wording, chosen from the operand's shape, and the report on the `!!` token.
 *
 * Deliberate differences from Go, each pinned in the golden data:
 * - Precision: the guards above match the operand by its resolved variable and receiver, so a
 *   same-named variable in another scope is a different value. Go matches by name and declaration
 *   lookup in the file.
 * - Recall: a stable smart cast that relies on a property of another Gradle module does not count, as in that
 *   module's own build. Go drops the compiler's verdict there and falls back to its flow guards, so it skips
 *   `if (x.p != null) x.p!!` on such a property; the checker reports it, as it does for a mutable property.
 * - Recall: Go's name-based idiom skips (Bundle and Intent getters, view bindings, proto fields)
 *   apply only when the compiler has no type for the operand, which never happens here, so the
 *   operand is judged by its type.
 */
internal object UnsafeCallOnNullableType : FirCheckNotNullCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnsafeCallOnNullableType"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val checkNotNullCallCheckers = setOf(UnsafeCallOnNullableType)
    }

    private const val DETEKT_MESSAGE =
        "Calling !! on a nullable type will throw a NullPointerException at runtime in case the value is null. It should be avoided."
    private const val PLAIN_MESSAGE = DETEKT_MESSAGE +
        " Prefer requireNotNull(x) { \"reason\" } or checkNotNull(x) { \"reason\" }; use a safe call (?.) only when null is a valid case."
    private const val SUPER_MESSAGE = DETEKT_MESSAGE +
        " Use checkNotNull(x) { \"reason\" } to document why a platform-type call result cannot be null."
    private const val COLUMN_MESSAGE = DETEKT_MESSAGE +
        " Use requireNotNull(x) { \"reason\" } on a value read from a column or cursor so a missing value reports what was expected."
    private const val ARGUMENT_MESSAGE = DETEKT_MESSAGE +
        " Use requireNotNull(x) { \"reason\" } on a call argument to document the precondition."
    private const val RX_MESSAGE = DETEKT_MESSAGE +
        " A mapper cannot return null, so consider requireNotNull() or checkNotNull() with a message instead."

    private const val MAX_REQUIRE_HOPS = 6
    private const val MAX_CHAIN_STEPS = 8
    private const val MAX_SIGNATURE = 500

    private val columnReadMarkers = listOf(
        "getString(", "getInt(", "getLong(", "getBlob(", "getDouble(", "getFloat(", "getShort(", "getColumnIndex", "cursor.",
    ).map { it.lowercase() }

    private val rxMapperOperators = setOf(
        "map", "flatMap", "concatMap", "switchMap", "flatMapSingle", "flatMapMaybe", "flatMapObservable",
        "flatMapPublisher", "scan", "flatMapIterable", "concatMapSingle", "switchMapSingle",
    )

    private val testSupportMarkers = listOf(
        "-testing/", "/testing/", "/test-utils/", "-test-utils/", "/test-fixtures/", "/testfixtures/", "/fakes/", "/fake/",
        "/mocks/", "/mock/", "/testing-ui/", "/ui-testing/", "/maestro-runner/", "/idling-resources/", "/idlingresources/",
        "/shared-instrumentation/", "/instrumentation-utils/", "/instrumentation-tests/", "/instrumentationtests/",
        "/android-test-utils/", "/androidtest-utils/", "/testutils/", "/testinternal/", "/testdebug/", "/testrelease/",
        "/testwrapper/", "/espressomodule.kt",
    )

    private val androidxPreviews = setOf("PreviewLightDark", "PreviewFontScale", "PreviewScreenSizes", "PreviewDynamicColors")
    private val previewNamePrefixes = listOf("preview", "sample", "fake", "mock", "stub", "fixture")
    private val previewNameSuffixes = listOf("preview", "sample", "fixture")

    private val transformCalls = setOf(
        "map", "mapNotNull", "mapIndexed", "flatMap", "forEach", "forEachIndexed", "associate", "associateBy",
        "associateWith", "sortedBy", "sortedByDescending", "groupBy", "onEach", "let",
    )
    private val filterCalls = setOf("filter", "filterKeys", "filterValues")
    private val preconditionConditionCalls = setOf("check", "require", "assert")
    private val preconditionNotNullCalls = setOf("requireNotNull", "checkNotNull")

    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)
    private val accessTokens = setOf(KtTokens.DOT, KtTokens.SAFE_ACCESS)

    // Node types where the `requireX` expression-body search stops, as Go stops at statements, lambdas and branches.
    private val requireBoundaries = setOf(
        KtNodeTypes.BLOCK, KtNodeTypes.FUNCTION_LITERAL, KtNodeTypes.LAMBDA_EXPRESSION, KtNodeTypes.IF, KtNodeTypes.WHEN,
        KtNodeTypes.TRY, KtNodeTypes.THEN, KtNodeTypes.ELSE, KtNodeTypes.BODY,
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirCheckNotNullCall) {
        val source = expression.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val postfix = source.lighterASTNode
        if (postfix.tokenType != KtNodeTypes.POSTFIX_EXPRESSION) return
        val operand = significantChildren(source, postfix).firstOrNull() ?: return
        val operandText = lightText(source, operand)
        if (!isNullable(expression.argument)) return
        if (isTestSupportPath() || isNamedKClassSimpleName(operandText)) return
        // map[key]!! is the sibling rule's concern.
        if (operandText.endsWith("]")) return
        if (isInsidePreviewOrSample(source, postfix)) return
        if (isRequireFunctionBody(source, postfix)) return
        if (isPreconditionGuarded(expression)) return
        if (isPostFilterSmartCast(operandText)) return
        report(operatorAnchor(source, postfix), message(source, postfix, operandText))
    }

    // The compiler's type for the operand: a platform type and a stable smart cast to non-null are not nullable,
    // an unstable smart cast is looked through, and a type parameter with a nullable bound can be null.
    context(context: CheckerContext)
    private fun isNullable(operand: FirExpression): Boolean {
        var expression = operand
        while (expression is FirSmartCastExpression && (!expression.isStable || reliesOnOtherModule(expression))) {
            expression = expression.originalExpression
        }
        return expression.resolvedType.lowerBoundIfFlexible().canBeNull(context.session)
    }

    // A stable smart cast on a property of another Gradle module is rejected by that module's own build. The check
    // compile reads a dependency without build output from source, where the property looks stable, so the module
    // directory (the path before `src/<source set>/kotlin`) of every property the cast relies on must be this file's.
    context(context: CheckerContext)
    private fun reliesOnOtherModule(cast: FirSmartCastExpression): Boolean {
        val own = moduleDirectory(context.containingFile?.path) ?: return false
        var current: FirExpression? = cast.originalExpression
        while (current != null) {
            current = when (current) {
                is FirSmartCastExpression -> current.originalExpression
                is FirPropertyAccessExpression -> {
                    val declared = current.calleeReference.toResolvedPropertySymbol()?.unwrapFakeOverrides()?.declaringSourcePath(context.session)
                    val module = moduleDirectory(declared)
                    if (module != null && module != own) return true
                    current.explicitReceiver
                }
                else -> null
            }
        }
        return false
    }

    private val sourceSetRoot = Regex("/src/[A-Za-z0-9_]+/(?:kotlin|java)/")

    private fun moduleDirectory(path: String?): String? {
        val normalized = path?.replace('\\', '/') ?: return null
        val match = sourceSetRoot.findAll(normalized).lastOrNull() ?: return null
        return normalized.substring(0, match.range.first)
    }

    private fun operatorAnchor(source: KtSourceElement, postfix: LighterASTNode): KtSourceElement {
        val operator = lightChildren(source, postfix).firstOrNull { it.tokenType == KtNodeTypes.OPERATION_REFERENCE }
        return operator?.let { lightSourceOf(it, source) } ?: source
    }

    // ---- Path and scope exemptions -----------------------------------------

    context(context: CheckerContext)
    private fun isTestSupportPath(): Boolean {
        if (isInTestFile()) return true
        val path = containingScanPath()?.replace('\\', '/')?.lowercase() ?: return false
        return testSupportMarkers.any { it in path }
    }

    private fun isNamedKClassSimpleName(text: String): Boolean {
        val suffix = "::class.simpleName"
        if (!text.endsWith(suffix)) return false
        val qualifier = text.removeSuffix(suffix)
        if (qualifier.isEmpty()) return false
        val parts = qualifier.split(".")
        if (parts.last().firstOrNull() !in 'A'..'Z') return false
        return parts.all { part ->
            part.isNotEmpty() && (part[0] == '_' || part[0] in 'A'..'Z' || part[0] in 'a'..'z') &&
                part.all { it == '_' || it in 'A'..'Z' || it in 'a'..'z' || it in '0'..'9' }
        }
    }

    // The nearest enclosing function names preview or sample scaffolding, or carries a preview annotation.
    context(context: CheckerContext)
    private fun isInsidePreviewOrSample(source: KtSourceElement, postfix: LighterASTNode): Boolean {
        val config = config()
        val wildcard = config["customPreviewWildcard"] as? Boolean ?: true
        val prefixes = (config["customPreviewPrefixes"] as? List<*>).orEmpty().filterIsInstance<String>()
        var node = source.treeStructure.getParent(postfix)
        while (node != null) {
            when (node.tokenType) {
                KtNodeTypes.FUN -> return isPreviewFunction(source, node, wildcard, prefixes)
                KtNodeTypes.CLASS, KtNodeTypes.OBJECT_DECLARATION ->
                    if (hasPreviewSignature(source, node, wildcard, prefixes)) return true
            }
            node = source.treeStructure.getParent(node)
        }
        return false
    }

    context(context: CheckerContext)
    private fun isPreviewFunction(source: KtSourceElement, fn: LighterASTNode, wildcard: Boolean, prefixes: List<String>): Boolean {
        val name = lightChildren(source, fn).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
            ?.let { lightText(source, it) }.orEmpty().lowercase()
        if (previewNamePrefixes.any { name.startsWith(it) } || previewNameSuffixes.any { name.endsWith(it) }) return true
        val modifiers = lightChildren(source, fn).firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST } ?: return false
        val text = lightText(source, modifiers)
        return textHasPreviewAnnotation(text, wildcard, prefixes) || usesMultipreviewAnnotation(text)
    }

    // A class or object whose annotations name a preview and that holds a function, as Go reads the declaration's text.
    context(context: CheckerContext)
    private fun hasPreviewSignature(source: KtSourceElement, declaration: LighterASTNode, wildcard: Boolean, prefixes: List<String>): Boolean {
        val modifiers = lightChildren(source, declaration).firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST } ?: return false
        val text = lightText(source, modifiers).take(MAX_SIGNATURE)
        if (!textHasPreviewAnnotation(text, wildcard, prefixes)) return false
        return "fun " in lightText(source, declaration)
    }

    private fun textHasPreviewAnnotation(text: String, wildcard: Boolean, prefixes: List<String>): Boolean {
        var rest = text
        while (true) {
            val at = rest.indexOf('@')
            if (at < 0) return false
            rest = rest.substring(at + 1)
            val end = rest.indexOfFirst { !(it in 'a'..'z' || it in 'A'..'Z' || it in '0'..'9' || it == '_' || it == '.' || it == ':') }
                .let { if (it < 0) rest.length else it }
            if (end > 0 && previewNameMatches(rest.substring(0, end), wildcard, prefixes)) return true
            rest = rest.substring(end)
        }
    }

    private fun annotationSimpleName(raw: String): String {
        var text = raw.removePrefix("@")
        text.indexOf(':').let { if (it >= 0) text = text.substring(it + 1) }
        text.indexOf('(').let { if (it >= 0) text = text.substring(0, it) }
        text = text.trim()
        text.lastIndexOf('.').let { if (it >= 0) text = text.substring(it + 1) }
        return text
    }

    private fun previewNameMatches(name: String, wildcard: Boolean, prefixes: List<String>): Boolean {
        val simple = annotationSimpleName(name)
        if (simple.isEmpty()) return false
        if (simple == "Preview" || simple in androidxPreviews) return true
        for (raw in prefixes) {
            val prefix = raw.trim()
            if (prefix.isNotEmpty() && (simple == prefix || simple == prefix + "Preview" || simple == prefix + "Previews")) return true
        }
        if (!wildcard) return false
        return simple.endsWith("Preview") || simple.endsWith("Previews") || simple.startsWith("Preview")
    }

    // An annotation class declared in this file and itself marked as a preview (a multipreview such as `@Themes`),
    // applied in [modifiers].
    @OptIn(DirectDeclarationsAccess::class)
    context(context: CheckerContext)
    private fun usesMultipreviewAnnotation(modifiers: String): Boolean {
        if ('@' !in modifiers) return false
        val file = context.containingElements.filterIsInstance<FirFile>().firstOrNull() ?: return false
        return file.declarations.any { multipreviewClassUsed(it, modifiers) }
    }

    @OptIn(DirectDeclarationsAccess::class)
    private fun multipreviewClassUsed(declaration: FirElement, modifiers: String): Boolean {
        if (declaration !is FirRegularClass) return false
        if (declaration.classKind == ClassKind.ANNOTATION_CLASS) {
            val source = declaration.source
            val classModifiers = source?.let { src ->
                lightChildren(src, src.lighterASTNode).firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST }?.let { lightText(src, it) }
            }.orEmpty()
            if (hasPreviewAnnotation(classModifiers) && usesAnnotationNamed(modifiers, declaration.name.asString())) return true
        }
        return declaration.declarations.any { multipreviewClassUsed(it, modifiers) }
    }

    private fun hasPreviewAnnotation(text: String): Boolean {
        var index = 0
        while (index < text.length) {
            if (text[index] != '@') {
                index++
                continue
            }
            val start = index + 1
            var end = start
            while (end < text.length && (text[end] in 'a'..'z' || text[end] in 'A'..'Z' || text[end] in '0'..'9' || text[end] == '_' || text[end] == '.')) end++
            if (end > start) {
                val simple = annotationSimpleName(text.substring(start, end))
                if (simple in androidxPreviews || simple.endsWith("Preview")) return true
            }
            index++
        }
        return false
    }

    private fun usesAnnotationNamed(text: String, name: String): Boolean {
        var index = 0
        while (index < text.length) {
            if (text[index] != '@') {
                index++
                continue
            }
            var end = index + 1
            while (end < text.length && (text[end] in 'a'..'z' || text[end] in 'A'..'Z' || text[end] in '0'..'9' || text[end] == '_' || text[end] == '.' || text[end] == ':')) end++
            if (annotationSimpleName(text.substring(index + 1, end)) == name) return true
            index++
        }
        return false
    }

    // `fun requireX(): T = field!!` documents the precondition in its name.
    private fun isRequireFunctionBody(source: KtSourceElement, postfix: LighterASTNode): Boolean {
        var node = source.treeStructure.getParent(postfix)
        var hops = 0
        var fn: LighterASTNode? = null
        while (node != null) {
            hops++
            if (hops > MAX_REQUIRE_HOPS) return false
            if (node.tokenType == KtNodeTypes.FUN) {
                fn = node
                break
            }
            if (node.tokenType in requireBoundaries) return false
            node = source.treeStructure.getParent(node)
        }
        fn ?: return false
        val name = lightChildren(source, fn).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }?.let { lightText(source, it) } ?: return false
        if (!name.startsWith("require")) return false
        if (name.length > "require".length && name["require".length] !in 'A'..'Z') return false
        // An expression body has the `=` token directly under the function and no block body.
        val children = significantChildren(source, fn)
        return children.any { it.tokenType == KtTokens.EQ } && children.none { it.tokenType == KtNodeTypes.BLOCK }
    }

    // ---- Message ------------------------------------------------------------

    context(context: CheckerContext)
    private fun message(source: KtSourceElement, postfix: LighterASTNode, operandText: String): String {
        if (insideRxMapper(source, postfix)) return RX_MESSAGE
        if (operandText.startsWith("super.")) return SUPER_MESSAGE
        val lower = operandText.lowercase()
        if (columnReadMarkers.any { it in lower }) return COLUMN_MESSAGE
        val parent = source.treeStructure.getParent(postfix)
        if (parent?.tokenType == KtNodeTypes.VALUE_ARGUMENT) return ARGUMENT_MESSAGE
        return PLAIN_MESSAGE
    }

    // The nearest enclosing lambda is the argument of an RxJava mapper operator, in a file that imports RxJava.
    context(context: CheckerContext)
    private fun insideRxMapper(source: KtSourceElement, postfix: LighterASTNode): Boolean {
        val file = context.containingElements.filterIsInstance<FirFile>().firstOrNull() ?: return false
        if (file.imports.none { it.importedFqName?.asString()?.startsWith("io.reactivex.") == true }) return false
        var node = source.treeStructure.getParent(postfix)
        while (node != null && node.tokenType != KtNodeTypes.FUNCTION_LITERAL) node = source.treeStructure.getParent(node)
        var call = node?.let { source.treeStructure.getParent(it) }
        while (call != null && call.tokenType != KtNodeTypes.CALL_EXPRESSION) call = source.treeStructure.getParent(call)
        call ?: return false
        val callee = lightChildren(source, call).firstOrNull { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION } ?: return false
        return lightText(source, callee) in rxMapperOperators
    }

    // ---- Preconditions ------------------------------------------------------

    // A value, identified by its variable and the receiver it is read from (a bound `this` symbol, another value,
    // or null for locals, parameters and top-level properties).
    private data class ValueKey(val variable: FirVariableSymbol<*>, val receiver: Any?)

    private fun valueKey(expression: FirExpression): ValueKey? {
        val access = unwrapArgument(expression).unwrapSmartcastExpression() as? FirPropertyAccessExpression ?: return null
        val variable = access.calleeReference.toResolvedVariableSymbol() ?: return null
        if (access.extensionReceiver != null) return null
        val receiver: Any? = when (val dispatch = access.dispatchReceiver) {
            null -> null
            is FirThisReceiverExpression -> dispatch.calleeReference.boundSymbol ?: return null
            else -> valueKey(dispatch) ?: return null
        }
        return ValueKey(variable, receiver)
    }

    private fun sameValue(a: FirExpression, b: FirExpression): Boolean {
        val left = valueKey(a)
        val right = valueKey(b)
        if (left != null && right != null) return left == right
        return false
    }

    private fun isNullConstant(expression: FirExpression): Boolean = isNullLiteral(unwrapArgument(expression))

    // An earlier statement in a block of the same function or lambda proves the operand non-null without a smart cast:
    // `x ?: return`, `check(x != null)`, `require(...)`, `assert(...)`, `requireNotNull(x)` or `checkNotNull(x)`.
    context(context: CheckerContext)
    private fun isPreconditionGuarded(call: FirCheckNotNullCall): Boolean {
        val path = context.containingElements
        for (i in path.indices.reversed()) {
            val element = path[i]
            if (element is FirNamedFunction || element is FirAnonymousFunction) return false
            if (element !is FirBlock) continue
            val child = path.getOrNull(i + 1) ?: continue
            for (statement in element.statements) {
                if (statement === child) break
                if (statementProvesNonNull(statement, call.argument)) return true
            }
        }
        return false
    }

    private fun statementProvesNonNull(statement: FirElement, operand: FirExpression): Boolean = when (statement) {
        is FirElvisExpression -> elvisExits(statement, operand)
        is FirProperty -> (statement.initializer as? FirElvisExpression)?.let { elvisExits(it, operand) } ?: false
        is FirFunctionCall -> preconditionCallProves(statement, operand)
        else -> false
    }

    private fun elvisExits(elvis: FirElvisExpression, operand: FirExpression): Boolean =
        sameValue(elvis.lhs, operand) && (elvis.rhs is FirJump<*> || elvis.rhs is FirThrowExpression)

    private fun preconditionCallProves(call: FirFunctionCall, operand: FirExpression): Boolean {
        val first = call.arguments.firstOrNull()?.let(::unwrapArgument) ?: return false
        return when (call.calleeReference.name.asString()) {
            in preconditionConditionCalls -> conditionTrueProvesNonNull(first, operand)
            in preconditionNotNullCalls -> sameValue(first, operand)
            else -> false
        }
    }

    private fun conditionTrueProvesNonNull(condition: FirExpression, operand: FirExpression): Boolean = when {
        condition is FirEqualityOperatorCall -> equalityFacts(condition, operand) == NullFact.NON_NULL
        condition is FirBooleanOperatorExpression && condition.kind == LogicOperationKind.AND ->
            conditionTrueProvesNonNull(condition.leftOperand, operand) || conditionTrueProvesNonNull(condition.rightOperand, operand)
        condition is FirBooleanOperatorExpression ->
            conditionTrueProvesNonNull(condition.leftOperand, operand) && conditionTrueProvesNonNull(condition.rightOperand, operand)
        else -> negated(condition)?.let { conditionFalseProvesNonNull(it, operand) } ?: false
    }

    private fun conditionFalseProvesNonNull(condition: FirExpression, operand: FirExpression): Boolean = when {
        condition is FirEqualityOperatorCall -> equalityFacts(condition, operand) == NullFact.NULL
        condition is FirFunctionCall && negated(condition) == null -> nullPredicateFalseProvesNonNull(condition, operand)
        condition is FirBooleanOperatorExpression && condition.kind == LogicOperationKind.OR ->
            conditionFalseProvesNonNull(condition.leftOperand, operand) || conditionFalseProvesNonNull(condition.rightOperand, operand)
        condition is FirBooleanOperatorExpression ->
            conditionFalseProvesNonNull(condition.leftOperand, operand) && conditionFalseProvesNonNull(condition.rightOperand, operand)
        else -> negated(condition)?.let { conditionTrueProvesNonNull(it, operand) } ?: false
    }

    // The operand of a `!x` prefix, which K2 reads as a `not` call.
    private fun negated(condition: FirExpression): FirExpression? {
        if (condition !is FirFunctionCall || condition.calleeReference.name.asString() != "not") return null
        return condition.explicitReceiver
    }

    private enum class NullFact { NON_NULL, NULL }

    // What `operand != null` / `operand == null` says about the operand when it is true.
    private fun equalityFacts(call: FirEqualityOperatorCall, operand: FirExpression): NullFact? {
        val args = call.arguments.map(::unwrapArgument)
        if (args.size != 2) return null
        val candidate = when {
            isNullConstant(args[1]) -> args[0]
            isNullConstant(args[0]) -> args[1]
            else -> return null
        }
        if (!sameValue(candidate, operand)) return null
        return when (call.operation) {
            FirOperation.NOT_EQ -> NullFact.NON_NULL
            FirOperation.EQ -> NullFact.NULL
            else -> null
        }
    }

    // `x.isNullOrEmpty()`, `x.isNullOrBlank()` and `TextUtils.isEmpty(x)` are false only for a non-null x.
    private fun nullPredicateFalseProvesNonNull(call: FirFunctionCall, operand: FirExpression): Boolean {
        val receiver = call.explicitReceiver
        return when (call.calleeReference.name.asString()) {
            "isNullOrEmpty", "isNullOrBlank" -> receiver != null && sameValue(receiver, operand)
            "isEmpty" -> {
                val isTextUtils = ((receiver as? FirQualifiedAccessExpression)?.calleeReference as? FirNamedReference)?.name?.asString() == "TextUtils"
                isTextUtils && call.arguments.firstOrNull()?.let { sameValue(it, operand) } == true
            }
            else -> false
        }
    }

    // ---- Post-filter smart cast ---------------------------------------------

    // `.filter { it.f != null }.map { it.f!! }` (or `.filterNotNull().map { it!! }`): the transform lambda's operand
    // was checked by an earlier filter in the same chain.
    context(context: CheckerContext)
    private fun isPostFilterSmartCast(operandText: String): Boolean {
        val base = operandText.removeSuffix(".")
        if (base != "it" && !base.startsWith("it.")) return false
        val field = base.removePrefix("it.")
        val path = context.containingElements
        val lambdaIndex = path.indices.reversed().firstOrNull { path[it] is FirAnonymousFunction || path[it] is FirNamedFunction }
        if (lambdaIndex == null || path[lambdaIndex] !is FirAnonymousFunction) return false
        for (i in lambdaIndex - 1 downTo 0) {
            val element = path[i]
            if (element is FirNamedFunction) return false
            if (element is FirFunctionCall && chainHasFilter(element, base, field)) return true
        }
        return false
    }

    private fun chainHasFilter(call: FirFunctionCall, base: String, field: String): Boolean {
        if (call.calleeReference.name.asString() !in transformCalls) return false
        if (!isChainTail(call)) return false
        var current: FirExpression? = receiverOf(call)
        repeat(MAX_CHAIN_STEPS) {
            val receiverCall = callOf(current)
            when {
                receiverCall != null -> {
                    val name = receiverCall.calleeReference.name.asString()
                    if (name == "filterNotNull" && base == "it") return true
                    if (name in filterCalls && filterLambdaGuardsField(receiverCall, field)) return true
                    current = receiverOf(receiverCall)
                }
                current is FirPropertyAccessExpression -> current = receiverOf(current)
                else -> return false
            }
        }
        return false
    }

    private fun callOf(expression: FirExpression?): FirFunctionCall? = when (expression) {
        is FirFunctionCall -> expression
        is FirSafeCallExpression -> expression.selector as? FirFunctionCall
        else -> null
    }

    private fun receiverOf(expression: FirQualifiedAccessExpression): FirExpression? =
        when (val receiver = expression.explicitReceiver) {
            is FirCheckedSafeCallSubject -> receiver.originalReceiverRef.value
            else -> receiver
        }

    // Go reads the callee of the outermost qualified expression, so the transform call must end its chain.
    private fun isChainTail(call: FirFunctionCall): Boolean {
        val source = call.source ?: return false
        val qualified = qualifiedCall(source) ?: return true
        val parent = source.treeStructure.getParent(qualified) ?: return true
        if (parent.tokenType !in qualifiedTypes) return true
        val first = significantChildren(source, parent, accessTokens).firstOrNull() ?: return true
        return !(first.startOffset == qualified.startOffset && first.endOffset == qualified.endOffset)
    }

    private fun filterLambdaGuardsField(filter: FirFunctionCall, field: String): Boolean {
        val lambda = filter.arguments.map(::unwrapArgument).filterIsInstance<FirAnonymousFunctionExpression>()
            .firstOrNull()?.anonymousFunction ?: return false
        val parameter = lambda.valueParameters.firstOrNull()?.name?.asString() ?: "it"
        var found = false
        lambda.body?.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                if (found) return
                if (element is FirEqualityOperatorCall && checksFieldNotNull(element, parameter, field)) {
                    found = true
                    return
                }
                element.acceptChildren(this)
            }
        })
        return found
    }

    private fun checksFieldNotNull(call: FirEqualityOperatorCall, parameter: String, field: String): Boolean {
        if (call.operation != FirOperation.NOT_EQ) return false
        val args = call.arguments.map(::unwrapArgument)
        if (args.size != 2 || !isNullConstant(args[1])) return false
        val access = args[0].unwrapSmartcastExpression() as? FirPropertyAccessExpression ?: return false
        if (access.calleeReference.name.asString() != field) return false
        val receiver = access.explicitReceiver?.unwrapSmartcastExpression() as? FirPropertyAccessExpression ?: return false
        return receiver.calleeReference.name.asString() == parameter
    }
}
