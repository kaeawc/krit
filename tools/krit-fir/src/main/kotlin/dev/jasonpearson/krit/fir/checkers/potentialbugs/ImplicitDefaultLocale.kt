package dev.jasonpearson.krit.fir.checkers.potentialbugs

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.containingScanPath
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

// Flags String calls that use the JVM's default locale without saying so:
// - the no-argument case conversions `toLowerCase()` / `toUpperCase()` /
//   `capitalize()` / `decapitalize()`: the kotlin.text extensions on String
//   (DEPRECATION_ERROR since Kotlin 2.1, so they only reach a clean compile
//   under `@Suppress("DEPRECATION_ERROR")`) and the java.lang.String members;
// - `String.format(pattern, args...)`: the stdlib `String.Companion.format`
//   and the JDK's static `java.lang.String.format(String, Object...)`;
// - `"pattern".format(args...)`: the stdlib `String.format(vararg args)`
//   extension called on a string literal or template, as Go requires.
// The overloads that take a Locale first are never reported.
//
// Mirrors the Go rule's evidence on top of FIR resolution:
// - `.gradle.kts` scripts and files whose scan path ends in `Table.kt`,
//   `Tables.kt`, `Dao.kt` or `DAO.kt` are skipped;
// - only calls with an explicit receiver are reported, on the line where the
//   whole qualified call starts (Go's call_expression);
// - a case conversion whose receiver's source text contains one of Go's
//   ASCII-invariant identifiers (`currencyCode`, `url`, `hex`, ...) is
//   skipped: a plain substring match, exactly as Go does it;
// - a format pattern made only of locale-independent conversions (`%s`, `%x`,
//   `%%`, `%n`, ...; Go's isLocaleInsensitiveFormat) is skipped: the receiver
//   literal of `"...".format(...)`, and for `String.format(...)` the first
//   argument when its text is a string literal, or when it names a property
//   whose initializer is a plain string literal. Go finds that property by
//   name, the first one in the file in source order, and so does this checker.
//
// Deliberate differences from Go, each pinned in the golden data:
// - Precision: Go matches the call by name and does not resolve it. FIR does
//   not report a project function or member of the same name, the
//   locale-independent `Char.toLowerCase()` / `Char.toUpperCase()`, or a call
//   that resolves to a Locale overload with a Locale held in a variable or
//   passed as null (Go only recognises an argument spelled `Locale.` /
//   `Locale(`).
// - Recall: FIR reports the static format however its receiver is spelled
//   (`kotlin.String.format`, `java.lang.String.format`, a typealias), calls
//   through an import alias, and calls whose first argument merely mentions
//   `Locale.` but is the pattern or a format argument
//   (`String.format(Locale.US.toString(), x)`), which still format with the
//   default locale. Go misses those.
internal object ImplicitDefaultLocale : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ImplicitDefaultLocale"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(ImplicitDefaultLocale)
    }

    private val kotlinText = FqName("kotlin.text")
    private val javaString = ClassId(FqName("java.lang"), Name.identifier("String"))
    private val javaLocale = ClassId(FqName("java.util"), Name.identifier("Locale"))
    private val stringCompanion = StandardClassIds.String.createNestedClassId(Name.identifier("Companion"))
    private val format = Name.identifier("format")
    private val stdlibCaseNames = setOf("toLowerCase", "toUpperCase", "capitalize", "decapitalize").map(Name::identifier).toSet()
    private val jdkCaseNames = setOf("toLowerCase", "toUpperCase").map(Name::identifier).toSet()

    // Go skips files whose path ends with one of these.
    private val skippedPathSuffixes = listOf(".gradle.kts", "Table.kt", "Tables.kt", "Dao.kt", "DAO.kt")

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

    private val qualifiedTypes = setOf(KtNodeTypes.DOT_QUALIFIED_EXPRESSION, KtNodeTypes.SAFE_ACCESS_EXPRESSION)

    private enum class Shape { CASE, STATIC_FORMAT, INSTANCE_FORMAT }

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val callee = expression.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
        val shape = shapeOf(callee) ?: return
        val path = containingScanPath()
        if (path != null && skippedPathSuffixes.any { path.endsWith(it) }) return

        val source = expression.source ?: return
        val tree = source.treeStructure
        val qualified = qualifiedCall(source) ?: return
        val parts = significantChildren(source, qualified)
        val receiver = parts.firstOrNull() ?: return
        val receiverText = tree.toString(receiver).toString()
        val anchor = lightSourceOf(qualified, source)

        when (shape) {
            Shape.CASE -> {
                if (asciiInvariantIdentifiers.any { receiverText.contains(it) }) return
                val name = callee.name.asString()
                report(
                    anchor,
                    "'$name()' called without explicit Locale. Use '$name(Locale.ROOT)' or '$name(Locale.getDefault())' to be explicit.",
                )
                return
            }
            Shape.INSTANCE_FORMAT -> {
                // Go only considers a string literal receiver.
                if (receiver.tokenType != KtNodeTypes.STRING_TEMPLATE) return
                if (isLocaleInsensitiveFormat(receiverText)) return
            }
            Shape.STATIC_FORMAT -> {
                val call = parts.last().takeIf { it.tokenType == KtNodeTypes.CALL_EXPRESSION } ?: return
                if (firstArgumentIsLocaleInsensitive(source, call)) return
            }
        }
        report(
            anchor,
            "'$receiverText.format(...)' uses implicit default locale for string formatting. Pass Locale explicitly, e.g. Locale.ROOT or Locale.US.",
        )
    }

    private fun shapeOf(callee: FirFunctionSymbol<*>): Shape? {
        val callableId = callee.callableId ?: return null
        val owner = callableId.classId
        val stdlib = owner == null && callableId.packageName == kotlinText
        val receiverType = callee.resolvedReceiverType?.lowerBoundIfFlexible()?.classId
        return when {
            callee.name == format -> {
                val first = callee.valueParameterSymbols.firstOrNull()
                if (first != null && first.resolvedReturnType.lowerBoundIfFlexible().classId == javaLocale) return null
                when {
                    owner == javaString -> Shape.STATIC_FORMAT
                    stdlib && receiverType == stringCompanion -> Shape.STATIC_FORMAT
                    stdlib && receiverType == StandardClassIds.String -> Shape.INSTANCE_FORMAT
                    else -> null
                }
            }
            callee.valueParameterSymbols.isNotEmpty() -> null
            owner == javaString && callee.name in jdkCaseNames -> Shape.CASE
            stdlib && callee.name in stdlibCaseNames && receiverType == StandardClassIds.String -> Shape.CASE
            else -> null
        }
    }

    // Go's exemptions for `String.format(first, ...)`, read from the first
    // value argument as written: a string literal whose pattern is
    // locale-independent, or a reference whose simple name is that of a
    // property initialized with such a literal.
    private fun firstArgumentIsLocaleInsensitive(source: KtSourceElement, call: LighterASTNode): Boolean {
        val tree = source.treeStructure
        val arguments = lightChildren(source, call).firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT_LIST }
            ?: return false
        val first = lightChildren(source, arguments).firstOrNull { it.tokenType == KtNodeTypes.VALUE_ARGUMENT }
            ?: return false
        val name = referenceSimpleName(source, argumentExpression(source, first))
        if (name != null) {
            val value = constStringPropertyValue(source, name)
            if (value != null && isLocaleInsensitiveFormat(value)) return true
        }
        val text = tree.toString(first).toString().trim()
        return text.startsWith("\"") && isLocaleInsensitiveFormat(text)
    }

    // The expression a value argument passes: the one after `=` for a named
    // argument, otherwise its first expression.
    private fun argumentExpression(source: KtSourceElement, argument: LighterASTNode): LighterASTNode? {
        val children = significantChildren(source, argument)
        val equals = children.indexOfFirst { it.tokenType == KtTokens.EQ }
        if (equals >= 0) return children.getOrNull(equals + 1)
        return children.firstOrNull { it.tokenType != KtNodeTypes.VALUE_ARGUMENT_NAME && it.tokenType != KtTokens.MUL }
    }

    // Go's flatReferenceSimpleName: the identifier of a simple name, or the
    // last selector of a qualified expression that ends in a property read.
    private fun referenceSimpleName(source: KtSourceElement, expression: LighterASTNode?): String? {
        var node = expression ?: return null
        while (node.tokenType == KtNodeTypes.PARENTHESIZED) {
            node = significantChildren(source, node).firstOrNull {
                it.tokenType != KtTokens.LPAR && it.tokenType != KtTokens.RPAR
            } ?: return null
        }
        if (node.tokenType in qualifiedTypes) {
            node = significantChildren(source, node).lastOrNull() ?: return null
        }
        if (node.tokenType != KtNodeTypes.REFERENCE_EXPRESSION) return null
        return source.treeStructure.toString(node).toString()
    }

    // Go's findConstStringPropertyValue: the first property declared anywhere
    // in the file, in source order, named [name] and initialized with a string
    // literal without templates; the literal's text without its escapes.
    private fun constStringPropertyValue(source: KtSourceElement, name: String): String? {
        val tree = source.treeStructure
        var result: String? = null
        fun visit(node: LighterASTNode) {
            if (result != null) return
            if (node.tokenType == KtNodeTypes.PROPERTY) {
                val children = significantChildren(source, node)
                val identifier = children.firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
                if (identifier != null && tree.toString(identifier).toString() == name) {
                    val equals = children.indexOfFirst { it.tokenType == KtTokens.EQ }
                    val initializer = if (equals >= 0) children.getOrNull(equals + 1) else null
                    if (initializer != null) plainStringContent(source, initializer)?.let { result = it; return }
                }
            }
            for (child in lightChildren(source, node)) visit(child)
        }
        visit(tree.root)
        return result
    }

    // The content of a string literal with no template entries, or null.
    private fun plainStringContent(source: KtSourceElement, node: LighterASTNode): String? {
        if (node.tokenType != KtNodeTypes.STRING_TEMPLATE) return null
        val tree = source.treeStructure
        val content = StringBuilder()
        for (child in lightChildren(source, node)) {
            when (child.tokenType) {
                KtNodeTypes.SHORT_STRING_TEMPLATE_ENTRY, KtNodeTypes.LONG_STRING_TEMPLATE_ENTRY -> return null
                KtNodeTypes.LITERAL_STRING_TEMPLATE_ENTRY -> content.append(tree.toString(child))
            }
        }
        return content.toString()
    }

    // Go's isLocaleInsensitiveFormat, character for character: true when the
    // pattern has only locale-independent conversions (%s %S %b %B %c %C %h %H
    // %x %X %o, plus %% and %n). Surrounding quotes are stripped first.
    internal fun isLocaleInsensitiveFormat(formatStr: String): Boolean {
        var s = formatStr
        if (s.length >= 2 && (s[0] == '"' || s[0] == '\'') && s[s.length - 1] == s[0]) {
            s = s.substring(1, s.length - 1)
        }
        s = s.removePrefix("\"\"").removeSuffix("\"\"")
        var i = 0
        while (i < s.length) {
            if (s[i] != '%') {
                i++
                continue
            }
            if (i + 1 >= s.length) return false
            val next = s[i + 1]
            if (next == '%' || next == 'n') {
                i += 2
                continue
            }
            var j = i + 1
            while (j < s.length && s[j].let { it in "-+ #0,(." || it in '0'..'9' }) {
                if (s[j] == ',') return false
                j++
            }
            if (j >= s.length) return false
            if (s[j] !in "sSbBcChHxXo") return false
            i = j + 1
        }
        return true
    }

    // The qualified expression (`r.f()` / `r?.f()`) whose selector is this
    // call, or null when the call has no explicit receiver. K2 gives a dot call
    // the whole qualified expression as its source; a safe call keeps the
    // selector call expression, so step up to its parent.
    private fun qualifiedCall(source: KtSourceElement): LighterASTNode? {
        val node = source.lighterASTNode
        if (node.tokenType in qualifiedTypes) return node
        if (node.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val parent = source.treeStructure.getParent(node) ?: return null
        if (parent.tokenType !in qualifiedTypes) return null
        val parts = significantChildren(source, parent)
        return parent.takeIf { parts.size > 1 && parts.last() == node }
    }

    private fun significantChildren(source: KtSourceElement, node: LighterASTNode): List<LighterASTNode> =
        lightChildren(source, node).filter {
            it.tokenType != KtTokens.WHITE_SPACE &&
                it.tokenType !in KtTokens.COMMENTS &&
                it.tokenType != KtTokens.DOT &&
                it.tokenType != KtTokens.SAFE_ACCESS
        }
}
