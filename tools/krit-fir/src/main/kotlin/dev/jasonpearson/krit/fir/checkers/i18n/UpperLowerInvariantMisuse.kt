package dev.jasonpearson.krit.fir.checkers.i18n

import com.intellij.lang.LighterASTNode
import com.intellij.util.diff.FlyweightCapableTreeStructure
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.qualifiedCall
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirDeclarationOrigin
import org.jetbrains.kotlin.fir.declarations.FirFunction
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirCheckedSafeCallSubject
import org.jetbrains.kotlin.fir.expressions.FirSafeCallExpression
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirAnonymousFunctionExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirSmartCastExpression
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.FirWrappedArgumentExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirAnonymousFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirNamedFunctionSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirReceiverParameterSymbol
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.text
import org.jetbrains.kotlin.toKtLightSourceElement
import org.jetbrains.kotlin.util.getChildren

// Flags the Kotlin 1.5+ stdlib case conversions `uppercase()` / `lowercase()`
// (kotlin.text, on String and Char) called without a Locale argument whose
// locale-invariant result is shown as user-facing text. Elsewhere the
// invariant conversion is the right one, so it is not reported.
//
// Mirrors the Go rule's evidence on top of FIR resolution:
// - the call is an argument of a UI text call (`Text(..)`, `setText(..)`,
//   `makeText(..)`, ...) or the right-hand side of an assignment to a UI text
//   property (`.text = ..`, `.title = ..`), up to the nearest function or
//   lambda; this gate is checked first, before the rest of the evidence;
// - the call resolves to kotlin.text.uppercase / kotlin.text.lowercase with no
//   arguments (the `(locale: Locale)` overloads are never flagged);
// - `.gradle.kts` scripts are skipped;
// - a call whose explicit receiver's source text contains one of Go's
//   ASCII-invariant identifiers (`currencyCode`, `url`, `mimeType`, `hex`,
//   ...) is skipped. The match is a plain substring match on the receiver
//   text, exactly as Go does it, so `ghostName` is skipped because it contains
//   `host`. A call on an implicit receiver gets the same exemption on the
//   text that names that receiver (see implicitReceiverText); when no text
//   names it, the call is not reported;
// - a receiver that holds a fixed token is skipped: a string literal without
//   templates, a `.name` read (a simple-name match, as in Go), and `toString()`
//   on an enum that does not override it (Go also skips an overriding enum
//   declared in the same file);
// - a call anywhere inside the arguments of `valueOf`, `enumValueOf` or
//   `valueOfOrNull` is skipped, up to the nearest function or lambda;
// - the finding is reported on the line where the whole qualified call starts
//   (the receiver's first line), as Go reports on its call_expression.
//
// Deliberate differences from Go, pinned by goldens:
// - Precision: Go matches any `<receiver>.uppercase()` / `.lowercase()` by
//   name. A project member or extension with that name (a class member, a
//   member or local extension, a same-package `String.uppercase()` or one
//   brought in by an explicit or star import, another function imported
//   `as uppercase`, or a local function-typed value invoked as `s.uppercase()`)
//   is not a stdlib case conversion and takes no Locale, so FIR does not
//   report it.
// - Recall: Go only sees calls with an explicit receiver and spelled
//   `uppercase` / `lowercase`. FIR also reports the stdlib calls made on an
//   implicit receiver (`with(s) { uppercase() }`, `s.run { uppercase() }`,
//   an extension body), through an import alias
//   (`import kotlin.text.uppercase as up`) and spelled with backticks
//   (`s.`uppercase`()`). The ASCII-invariant exemption, a user opt-out,
//   still applies to the implicit-receiver calls, so
//   `with(currencyCode) { uppercase() }` is not reported.
internal object UpperLowerInvariantMisuse : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UpperLowerInvariantMisuse"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(UpperLowerInvariantMisuse)
    }

    private val kotlinText = FqName("kotlin.text")
    private val kotlinPackage = FqName("kotlin")

    // Stdlib scope functions whose lambda's receiver is the call's explicit
    // receiver (`with` takes it as its first argument instead).
    private val receiverLambdaScopes = setOf("run", "apply")
    private val methodNames = setOf("uppercase", "lowercase")
    private val valueOfNames = setOf("valueOf", "enumValueOf", "valueOfOrNull")

    // Calls whose arguments, and properties whose assigned value, become text a user reads (as in Go).
    private val uiSinkCalls = setOf(
        "setText", "setTitle", "setSubtitle", "setHint", "setContentDescription", "setMessage",
        "setLabel", "setError", "setSummary", "makeText", "Text", "BasicText", "AnnotatedString",
    )
    private val uiSinkProperties = setOf(
        "text", "title", "subtitle", "hint", "contentDescription", "label", "error", "summary",
        "message", "headline", "placeholder",
    )

    // Go's containsASCIIInvariantIdentifier list, matched as substrings of
    // the receiver's source text.
    private val asciiInvariantIdentifiers = listOf(
        "currencyCode", "currency", "isoCode", "countryCode", "languageCode",
        "iban", "IBAN",
        "mimeType", "contentType", "MIME",
        "protocol", "scheme", "host", "uri", "URI", "url", "URL",
        "uuid", "UUID", "guid", "GUID",
        "serviceId", "deviceId",
        "cipher", "algorithm", "digest",
        "columnName", "columnNames", "tableName", "indexName",
        "hex", "Hex", "toHexString", "toHex",
        "verb", "httpMethod", "method", "requestMethod",
    )


    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() ?: return
        val callableId = callee.callableId ?: return
        if (callableId.packageName != kotlinText || callableId.className != null) return
        val name = callableId.callableName.asString()
        if (name !in methodNames) return
        if (expression.argumentList.arguments.isNotEmpty()) return
        if (context.containingFile?.name?.endsWith(".gradle.kts") == true) return

        val source = expression.source ?: return
        if (!flowsToUiText(expression, source)) return
        val qualified = qualifiedCall(source)
        val receiverText = if (qualified != null) {
            val receiver = significantChildren(qualified, source.treeStructure).firstOrNull() ?: return
            source.treeStructure.toString(receiver).toString()
        } else {
            implicitReceiverText(expression) ?: return
        }
        if (asciiInvariantIdentifiers.any { receiverText.contains(it) }) return
        if (qualified != null && isInvariantTokenReceiver(source, qualified, expression)) return
        if (isValueOfArgument(expression)) return
        report(qualified?.let { sourceOf(it, source) } ?: source, message(name))
    }

    // Receivers that hold fixed ASCII tokens, as in Go: a string literal without
    // templates, a `.name` read, and the toString() of an enum.
    context(context: CheckerContext)
    private fun isInvariantTokenReceiver(source: KtSourceElement, qualified: LighterASTNode, call: FirFunctionCall): Boolean {
        val tree = source.treeStructure
        val receiver = significantChildren(qualified, tree).firstOrNull() ?: return false
        val text = tree.toString(receiver).toString()
        return when (receiver.tokenType) {
            KtNodeTypes.STRING_TEMPLATE -> '$' !in text
            KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION -> {
                val selector = significantChildren(receiver, tree).lastOrNull()
                (selector?.tokenType == KtNodeTypes.REFERENCE_EXPRESSION && tree.toString(selector).toString() == "name") ||
                    isEnumToString(call.explicitReceiver)
            }
            else -> isEnumToString(call.explicitReceiver)
        }
    }

    // `toString()` on an enum that does not override it returns the entry name.
    @OptIn(DirectDeclarationsAccess::class)
    context(context: CheckerContext)
    private fun isEnumToString(receiver: FirExpression?): Boolean {
        var expression = receiver
        while (true) {
            expression = when (expression) {
                is FirSmartCastExpression -> expression.originalExpression
                is FirSafeCallExpression -> expression.selector as? FirExpression
                is FirCheckedSafeCallSubject -> expression.originalReceiverRef.value
                else -> break
            }
        }
        val call = expression as? FirFunctionCall ?: return false
        if (call.calleeReference.name.asString() != "toString" || call.argumentList.arguments.isNotEmpty()) return false
        val owner = call.dispatchReceiver ?: call.explicitReceiver ?: return false
        val enumClass = (owner.resolvedType.fullyExpandedType() as? ConeClassLikeType)
            ?.lookupTag?.toClassSymbol(context.session)
        if (enumClass?.classKind != ClassKind.ENUM_CLASS) return false
        // Only an override declared in this file counts, as in Go. Another file's override is source in a full
        // compile but a class file in a slice compile, so ignoring it keeps both compiles equal.
        val symbol = call.calleeReference.toResolvedCallableSymbol() ?: return false
        if (symbol.origin != FirDeclarationOrigin.Source) return true
        val outermost = enumClass.classId.outermostClassId
        val file = context.containingElements.firstNotNullOfOrNull { it as? FirFile } ?: return true
        return file.declarations.none { it is FirRegularClass && it.symbol.classId == outermost }
    }

    // True when [call] sits in the arguments of a UI text call or the value of a UI text property assignment.
    context(context: CheckerContext)
    private fun flowsToUiText(call: FirFunctionCall, source: KtSourceElement): Boolean {
        for (element in context.containingElements.asReversed()) {
            when {
                element === call -> continue
                element is FirFunctionCall ->
                    if (element.calleeReference.name.asString() in uiSinkCalls &&
                        element.argumentList.arguments.any { it.source.contains(source) }
                    ) return true
                element is FirVariableAssignment ->
                    if (((element.lValue as? FirPropertyAccessExpression)?.calleeReference?.name?.asString() in uiSinkProperties) &&
                        element.rValue.source.contains(source)
                    ) return true
                element is FirFunction || element is FirClass -> return false
            }
        }
        return false
    }

    private fun KtSourceElement?.contains(inner: KtSourceElement): Boolean =
        this != null && startOffset <= inner.startOffset && inner.endOffset <= endOffset

    // The case of a value passed to valueOf-style lookups matches a fixed constant name.
    context(context: CheckerContext)
    private fun isValueOfArgument(call: FirFunctionCall): Boolean {
        for (element in context.containingElements.asReversed()) {
            when {
                element === call -> continue
                element is FirFunctionCall && element.calleeReference.name.asString() in valueOfNames -> return true
                element is FirFunction || element is FirClass -> return false
            }
        }
        return false
    }

    // The text that names the implicit receiver of [call], for the
    // ASCII-invariant exemption Go applies to receiver text:
    // - in the lambda of a stdlib scope function, the receiver argument as
    //   written: `currencyCode` for `with(currencyCode) { .. }`,
    //   `currencyCode.run { .. }` and `currencyCode.apply { .. }`;
    // - in an extension function or property, its name, the label of its
    //   receiver (`this@hexUpper` in `fun String.hexUpper()`).
    // Null when nothing names the receiver (another lambda with receiver, a
    // scope function called on an implicit receiver).
    context(context: CheckerContext)
    private fun implicitReceiverText(call: FirFunctionCall): String? {
        var receiver = call.extensionReceiver ?: return null
        while (receiver is FirSmartCastExpression) receiver = receiver.originalExpression
        val thisReceiver = receiver as? FirThisReceiverExpression ?: return null
        val parameter = thisReceiver.calleeReference.boundSymbol as? FirReceiverParameterSymbol ?: return null
        return when (val owner = parameter.containingDeclarationSymbol) {
            is FirAnonymousFunctionSymbol -> scopeFunctionReceiverText(owner)
            is FirNamedFunctionSymbol -> owner.name.asString()
            is FirPropertySymbol -> owner.name.asString()
            else -> null
        }
    }

    // The receiver argument of the stdlib scope function call that takes
    // [lambda], as written.
    context(context: CheckerContext)
    private fun scopeFunctionReceiverText(lambda: FirAnonymousFunctionSymbol): String? {
        val call = context.callsOrAssignments.asReversed().firstNotNullOfOrNull { statement ->
            (statement as? FirFunctionCall)?.takeIf { candidate ->
                candidate.argumentList.arguments.any { it.passesLambda(lambda) }
            }
        } ?: return null
        val callableId = call.calleeReference.toResolvedCallableSymbol()?.callableId ?: return null
        if (callableId.packageName != kotlinPackage || callableId.className != null) return null
        val receiver = when (callableId.callableName.asString()) {
            "with" -> call.argumentList.arguments.firstOrNull { !it.passesLambda(lambda) }
            in receiverLambdaScopes -> call.explicitReceiver
            else -> null
        }
        return receiver?.source?.text?.toString()
    }

    private fun FirExpression.passesLambda(lambda: FirAnonymousFunctionSymbol): Boolean {
        val value = if (this is FirWrappedArgumentExpression) expression else this
        return value is FirAnonymousFunctionExpression && value.anonymousFunction.symbol == lambda
    }

    private fun message(name: String) =
        "'$name()' is locale-invariant but its result is shown as user-facing text. Use a display-locale variant such as '$name(Locale.getDefault())'."

    // A source element for [node], a node in [anchor]'s tree, keeping the
    // anchor's offset shift between tree offsets and file offsets.
    private fun sourceOf(node: LighterASTNode, anchor: KtSourceElement): KtSourceElement {
        if (node == anchor.lighterASTNode) return anchor
        val shift = anchor.startOffset - anchor.lighterASTNode.startOffset
        return node.toKtLightSourceElement(
            anchor.treeStructure,
            startOffset = node.startOffset + shift,
            endOffset = node.endOffset + shift,
        )
    }

    private fun significantChildren(
        node: LighterASTNode,
        tree: FlyweightCapableTreeStructure<LighterASTNode>,
    ): List<LighterASTNode> =
        node.getChildren(tree).filter {
            it.tokenType != KtTokens.WHITE_SPACE &&
                it.tokenType !in KtTokens.COMMENTS &&
                it.tokenType != KtTokens.DOT &&
                it.tokenType != KtTokens.SAFE_ACCESS
        }
}
