package dev.jasonpearson.krit.fir.checkers.security

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.assignedAccess
import dev.jasonpearson.krit.fir.support.unwrapArgument
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.expression.ExpressionCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.expression.FirFunctionCallChecker
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.expressions.resolvedArgumentMapping
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.symbols.impl.FirFunctionSymbol
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.types.ConstantValueKind

// Flags a `newInstance()` or `newFactory()` call on a JAXP XML parser factory (DocumentBuilderFactory,
// SAXParserFactory, XMLInputFactory, TransformerFactory, SchemaFactory) when the callable that creates it
// parses XML and applies no XXE hardening after the creation.
//
// Mirrors the Go XmlExternalEntity rule:
// - The scope is the nearest named function around the call (lambdas and anonymous functions belong to it),
//   else the nearest class or object (property initializers, init blocks, constructors, accessors), else the
//   file. Everything in the scope that starts after the creation counts, nested functions and lambdas
//   included. A call chained on the factory (`newInstance().newDocumentBuilder().parse(x)`) counts too.
// - The scope parses when it calls `parse`, `createXMLStreamReader`, `createXMLEventReader`,
//   `createFilteredReader` or `newSchema`, or `transform` with a StreamSource or SAXSource argument.
// - It is hardened when it sets one of the disabling features or properties to the literal that disables it
//   (`disallow-doctype-decl` true, the external entity features false, an ACCESS_EXTERNAL_* property to "",
//   an XMLInputFactory DTD or external entity property to false, FEATURE_SECURE_PROCESSING on a
//   TransformerFactory or SchemaFactory), or turns off both `isXIncludeAware` and `isExpandEntityReferences`.
//   A key is a string literal or the name of a constant such as `XMLConstants.ACCESS_EXTERNAL_DTD`.
//
// Differences from Go, pinned in the golden data:
// - Recall: the factory is identified by resolution, so an import alias and a type alias are reported; Go
//   needs the receiver spelled with the JAXP class name. `setXIncludeAware(false)` and
//   `setExpandEntityReferences(false)` count as the property assignments they stand for, so Go's report on a
//   factory hardened that way is dropped.
// - Precision: only calls that resolve to a javax.xml or org.xml type count as hardening or parsing. Go
//   matches by name, so it takes a `setFeature` on a project class as hardening and a `parse` on any
//   non-capitalised receiver as parsing. A factory class from another package with the same simple name
//   (a project class `DocumentBuilderFactory`) is not a JAXP factory.
internal object XmlExternalEntity : FirFunctionCallChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "XmlExternalEntity"
    override val expressionCheckers = object : ExpressionCheckers() {
        override val functionCallCheckers = setOf(XmlExternalEntity)
    }

    private const val MESSAGE =
        "XML factory created without XXE-disabling features. Disable DOCTYPE/external entities before parsing untrusted XML."

    private const val SECURE_PROCESSING = "FEATURE_SECURE_PROCESSING"
    private const val SECURE_PROCESSING_URL_PREFIX = "http://javax.xml.XMLConstants/feature/secure-processing"

    // Simple class name -> the package that declares the factory.
    private val factoryPackages = mapOf(
        "DocumentBuilderFactory" to "javax.xml.parsers",
        "SAXParserFactory" to "javax.xml.parsers",
        "XMLInputFactory" to "javax.xml.stream",
        "TransformerFactory" to "javax.xml.transform",
        "SchemaFactory" to "javax.xml.validation",
    )
    private val creationNames = setOf("newInstance", "newFactory")
    private val parseNames = setOf("parse", "createXMLStreamReader", "createXMLEventReader", "createFilteredReader", "newSchema")
    private val configNames = setOf("setFeature", "setProperty", "setAttribute")
    private val secureProcessingFactories = setOf("TransformerFactory", "SchemaFactory")

    // Feature URL -> the value that disables the unsafe behavior.
    private val featureHardening = mapOf(
        "http://apache.org/xml/features/disallow-doctype-decl" to true,
        "http://xml.org/sax/features/external-general-entities" to false,
        "http://xml.org/sax/features/external-parameter-entities" to false,
        "http://apache.org/xml/features/nonvalidating/load-external-dtd" to false,
    )
    private val accessExternalKeys = setOf(
        "ACCESS_EXTERNAL_DTD",
        "ACCESS_EXTERNAL_SCHEMA",
        "ACCESS_EXTERNAL_STYLESHEET",
        "http://javax.xml.XMLConstants/property/accessExternalDTD",
        "http://javax.xml.XMLConstants/property/accessExternalSchema",
        "http://javax.xml.XMLConstants/property/accessExternalStylesheet",
    )
    private val inputFactoryKeys = setOf(
        "SUPPORT_DTD",
        "IS_SUPPORTING_EXTERNAL_ENTITIES",
        "javax.xml.stream.supportDTD",
        "javax.xml.stream.isSupportingExternalEntities",
    )
    private val xIncludeNames = setOf("isXIncludeAware", "setXIncludeAware")
    private val expandEntityNames = setOf("isExpandEntityReferences", "setExpandEntityReferences")
    private val transformerId = ClassId(FqName("javax.xml.transform"), Name.identifier("Transformer"))
    private val sourceIds = setOf(
        ClassId(FqName("javax.xml.transform.stream"), Name.identifier("StreamSource")),
        ClassId(FqName("javax.xml.transform.sax"), Name.identifier("SAXSource")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(expression: FirFunctionCall) {
        val factory = factoryName(expression) ?: return
        val source = expression.source ?: return
        val scope = scopeOf(context.containingElements) ?: return
        val facts = ScopeFacts(factory, source.startOffset, source.endOffset, context.session)
        scope.accept(facts)
        if (facts.hardened || !facts.parses) return
        report(source, MESSAGE)
    }

    // The simple name of the JAXP factory this call creates, or null for any other call.
    private fun factoryName(call: FirFunctionCall): String? {
        val callee = call.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return null
        if (callee.callableId.callableName.asString() !in creationNames) return null
        val owner = callee.callableId.classId ?: return null
        val name = owner.shortClassName.asString()
        return name.takeIf { factoryPackages[it] == owner.packageFqName.asString() }
    }

    private fun scopeOf(path: List<FirElement>): FirElement? =
        path.lastOrNull { it is FirNamedFunction || it is FirRegularClass } ?: path.firstOrNull { it is FirFile }

    private fun isXmlType(classId: ClassId?): Boolean {
        val pkg = classId?.packageFqName?.asString() ?: return false
        return pkg == "javax.xml" || pkg.startsWith("javax.xml.") || pkg == "org.xml" || pkg.startsWith("org.xml.")
    }

    private class ScopeFacts(
        private val factory: String,
        private val start: Int,
        private val end: Int,
        private val session: FirSession,
    ) : FirVisitorVoid() {
        var hardened = false
            private set
        var parses = false
            private set
        private var xInclude = false
        private var expandEntities = false

        override fun visitElement(element: FirElement) {
            if (!hardened || !parses) inspect(element)
            element.acceptChildren(this)
        }

        private fun inspect(element: FirElement) {
            val source = element.source ?: return
            when (element) {
                is FirFunctionCall -> {
                    if (source.startOffset > start) inspectHardening(element)
                    if (source.startOffset > start || (source.startOffset == start && source.endOffset > end)) {
                        inspectParse(element)
                    }
                }
                is FirVariableAssignment -> if (source.startOffset > start) inspectAssignment(element)
            }
            if (xInclude && expandEntities) hardened = true
        }

        private fun inspectHardening(call: FirFunctionCall) {
            val callee = call.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
            val name = callee.callableId.callableName.asString()
            if (!isXmlMember(call)) return
            when (name) {
                in configNames -> if (isHardening(call, callee, name)) hardened = true
                in xIncludeNames -> if (argumentIsFalse(call, callee)) xInclude = true
                in expandEntityNames -> if (argumentIsFalse(call, callee)) expandEntities = true
            }
        }

        private fun inspectAssignment(assignment: FirVariableAssignment) {
            val access = assignedAccess(assignment) ?: return
            val name = access.calleeReference.toResolvedCallableSymbol()?.name?.asString() ?: return
            if (name !in xIncludeNames && name !in expandEntityNames) return
            if (!isXmlMember(access) || !isFalse(assignment.rValue)) return
            if (name in xIncludeNames) xInclude = true else expandEntities = true
        }

        private fun inspectParse(call: FirFunctionCall) {
            val callee = call.calleeReference.toResolvedCallableSymbol() as? FirFunctionSymbol<*> ?: return
            val name = callee.callableId.callableName.asString()
            if (!isXmlMember(call)) return
            if (name in parseNames) {
                parses = true
            } else if (name == "transform" && callee.callableId.classId == transformerId) {
                if (call.argumentList.arguments.any { isStreamOrSaxSource(it) }) parses = true
            }
        }

        // The class that declares the member, or the class of its dispatch receiver for an inherited member.
        private fun isXmlMember(access: FirQualifiedAccessExpression): Boolean {
            val symbol = access.calleeReference.toResolvedCallableSymbol() ?: return false
            if (isXmlType(symbol.callableId?.classId)) return true
            val receiver = access.dispatchReceiver ?: return false
            return isXmlType(receiver.resolvedType.fullyExpandedType(session).lowerBoundIfFlexible().classId)
        }

        private fun isStreamOrSaxSource(argument: FirExpression): Boolean =
            argument.resolvedType.classId in sourceIds

        private fun isFalse(expression: FirExpression): Boolean = booleanOf(unwrapArgument(expression)) == false

        private fun argumentAt(call: FirFunctionCall, callee: FirFunctionSymbol<*>, index: Int): FirExpression? {
            val parameter = callee.valueParameterSymbols.getOrNull(index) ?: return null
            return call.resolvedArgumentMapping?.entries?.firstOrNull { it.value.symbol == parameter }?.key
        }

        private fun argumentIsFalse(call: FirFunctionCall, callee: FirFunctionSymbol<*>): Boolean =
            argumentAt(call, callee, 0)?.let(::isFalse) == true

        // Go's xxeCallIsHardening: the first argument names the setting, the second is its literal value.
        private fun isHardening(call: FirFunctionCall, callee: FirFunctionSymbol<*>, name: String): Boolean {
            val key = keyOf(argumentAt(call, callee, 0) ?: return false) ?: return false
            val value = unwrapArgument(argumentAt(call, callee, 1) ?: return false)
            return when {
                name == "setFeature" && key == SECURE_PROCESSING ->
                    booleanOf(value) == true && factory in secureProcessingFactories
                name == "setFeature" -> featureHardening[key]?.let { booleanOf(value) == it } == true
                key in accessExternalKeys -> stringOf(value) == ""
                key in inputFactoryKeys -> booleanOf(value) == false
                else -> false
            }
        }

        // A string literal's content, or the name of the constant the argument reads.
        private fun keyOf(argument: FirExpression): String? {
            val expression = unwrapArgument(argument)
            stringOf(expression)?.let { return if (it.startsWith(SECURE_PROCESSING_URL_PREFIX)) SECURE_PROCESSING else it }
            return (expression as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()?.name?.asString()
        }

        private fun stringOf(expression: FirExpression): String? =
            (expression as? FirLiteralExpression)?.takeIf { it.kind == ConstantValueKind.String }?.value as? String

        private fun booleanOf(expression: FirExpression): Boolean? =
            (expression as? FirLiteralExpression)?.takeIf { it.kind == ConstantValueKind.Boolean }?.value as? Boolean
    }
}
