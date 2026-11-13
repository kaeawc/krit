package dev.jasonpearson.krit.fir.checkers.style

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.frameworkInjectedAnnotations
import dev.jasonpearson.krit.fir.support.goFullMatchRegex
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
import dev.jasonpearson.krit.fir.support.significantChildren
import dev.jasonpearson.krit.fir.support.unusedMemberSuppressions
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirValueParameter
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.references.FirResolvedNamedReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens

// Flags a private property, a private constructor property or a plain primary-constructor parameter
// that nothing in its file refers to, with the Go rule's scope and message
// (`Private property `x` is unused.`, on the declaration's name).
//
// Go looks for the name anywhere else in the file, so a same-named property of another class, a
// parameter or a method call keeps it quiet. This checker reads resolution instead: the declaration is
// used when some resolved reference in the file (a read, a write, a string template, a callable
// reference, a delegate) points at its symbol. A private declaration is visible only in its file, so
// the file holds every use. A constructor property is one declaration for two symbols, because an
// initializer or `init` block sees the parameter and a method sees the property.
//
// Not reported, like Go: a declaration that refers to itself only from its own text, a framework-injected
// one (`@Inject`, `@Mock`, ...) or one marked `@Suppress("unused")`, a name matching the `allowedNames`
// option (a full match, like Go), the `TAG` constant built by a `tag()` call, a property whose
// initializer calls something that registers, starts or subscribes (`launch`, `register*`, `observe*`,
// ...) because deleting it would delete that effect, and the constructor parameters of an `annotation`,
// `expect`, `actual`, `data`, `value` or `inline` class.
internal object UnusedPrivateProperty : FirFileChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnusedPrivateProperty"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(UnusedPrivateProperty)
    }

    // The default of the option in config/default-krit.yml.
    private const val DEFAULT_ALLOWED = "ignored|expected|serialVersionUID"

    private val sideEffectCall = Regex(
        "(subscribe\\w*|launch\\w*|async|register\\w*|observe\\w*|attach\\w*|listen\\w*|enqueue|schedule\\w*|collect|" +
            "connect|bindService|start|startService|add\\w*(Listener|Observer|Callback))",
    )

    private val exemptClassModifiers = listOf(
        KtTokens.ANNOTATION_KEYWORD, KtTokens.EXPECT_KEYWORD, KtTokens.ACTUAL_KEYWORD, KtTokens.DATA_KEYWORD,
        KtTokens.VALUE_KEYWORD, KtTokens.INLINE_KEYWORD,
    )

    private class Candidate(val name: String, val symbols: List<FirBasedSymbol<*>>, val anchor: KtSourceElement)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        val allowed = allowedNames()
        val candidates = ArrayList<Candidate>()
        val referenced = HashSet<FirBasedSymbol<*>>()
        // The symbols of the property whose own text is being walked: Go ignores a name inside its own declaration.
        val inside = HashSet<FirBasedSymbol<*>>()
        declaration.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                when (element) {
                    is FirResolvedNamedReference -> addReference(element.resolvedSymbol, inside, referenced)
                    is FirConstructor -> if (element.isPrimary) {
                        element.valueParameters.mapNotNullTo(candidates) { plainParameter(it, allowed) }
                    }
                    is FirProperty -> {
                        val candidate = propertyCandidate(element, allowed)
                        if (candidate != null) candidates += candidate
                        // The generated initializer of a constructor property reads the parameter it came from.
                        if (element.source?.kind == KtFakeSourceElementKind.PropertyFromParameter) return
                        if (candidate != null) {
                            // The accessors of a delegated property pass its own symbol and delegate field around.
                            val own = listOfNotNull<FirBasedSymbol<*>>(element.symbol, element.delegateFieldSymbol).filter { inside.add(it) }
                            element.acceptChildren(this)
                            inside.removeAll(own.toSet())
                            return
                        }
                    }
                }
                element.acceptChildren(this)
            }
        })
        for (candidate in candidates) {
            if (candidate.symbols.none { it in referenced }) {
                report(candidate.anchor, "Private property `${candidate.name}` is unused.")
            }
        }
    }

    private fun addReference(symbol: FirBasedSymbol<*>, inside: Set<FirBasedSymbol<*>>, referenced: MutableSet<FirBasedSymbol<*>>) {
        if (symbol in inside) return
        // A member read through a substituted receiver resolves to a fake override of the declaration.
        val declared = if (symbol is FirCallableSymbol<*>) symbol.unwrapFakeOverrides() else symbol
        if (declared in inside) return
        referenced += symbol
        referenced += declared
    }

    private fun allowedNames(): Regex {
        val configured = config()["allowedNames"] as? String
        return configured?.let(::goFullMatchRegex) ?: requireNotNull(goFullMatchRegex(DEFAULT_ALLOWED))
    }

    private fun propertyCandidate(property: FirProperty, allowed: Regex): Candidate? {
        // A local variable cannot be private, and a member of an anonymous object counts as local, so visibility decides.
        if (property.visibility != Visibilities.Private) return null
        val source = property.source ?: return null
        val fromParameter = source.kind == KtFakeSourceElementKind.PropertyFromParameter
        if (!fromParameter && (source.kind !is KtRealSourceElementKind || source.elementType != KtNodeTypes.PROPERTY)) return null
        if (fromParameter && !isReportableParameter(source)) return null
        val node = source.lighterASTNode
        val identifier = lightChildren(source, node).firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        val name = lightText(source, identifier).trim('`')
        if (name.isEmpty() || allowed.matches(name) || isExempt(source, node)) return null
        if (!fromParameter && (isTagFromTagCall(source, node, name) || initializerHasSideEffects(source, node))) return null
        val symbols = mutableListOf<FirBasedSymbol<*>>(property.symbol)
        if (fromParameter) {
            // The generated initializer reads the constructor parameter the property came from.
            (property.initializer as? FirQualifiedAccessExpression)?.calleeReference?.toResolvedCallableSymbol()?.let { symbols += it }
        }
        return Candidate(name, symbols, lightSourceOf(identifier, source))
    }

    private fun plainParameter(parameter: FirValueParameter, allowed: Regex): Candidate? {
        val source = parameter.source ?: return null
        if (source.kind !is KtRealSourceElementKind || source.elementType != KtNodeTypes.VALUE_PARAMETER) return null
        val node = source.lighterASTNode
        val children = lightChildren(source, node)
        // A `val`/`var` parameter is a property, judged with the other properties.
        if (children.any { it.tokenType == KtTokens.VAL_KEYWORD || it.tokenType == KtTokens.VAR_KEYWORD }) return null
        if (!isReportableParameter(source)) return null
        val identifier = children.firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        val name = lightText(source, identifier).trim('`')
        if (name.isEmpty() || allowed.matches(name) || isExempt(source, node)) return null
        return Candidate(name, listOf(parameter.symbol), lightSourceOf(identifier, source))
    }

    // A parameter of a primary constructor whose class is not an annotation, expect/actual, data, value or inline class.
    private fun isReportableParameter(source: KtSourceElement): Boolean {
        val tree = source.treeStructure
        val list = tree.getParent(source.lighterASTNode) ?: return false
        val constructor = tree.getParent(list)?.takeIf { it.tokenType == KtNodeTypes.PRIMARY_CONSTRUCTOR } ?: return false
        val owner = tree.getParent(constructor)?.takeIf { it.tokenType == KtNodeTypes.CLASS } ?: return false
        val modifiers = lightChildren(source, owner).firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST } ?: return true
        return lightChildren(source, modifiers).none { it.tokenType in exemptClassModifiers }
    }

    // Go reads the annotation text: a framework annotation, or a Suppress that names the unused-member family.
    private fun isExempt(source: KtSourceElement, node: LighterASTNode): Boolean {
        val modifiers = lightChildren(source, node).firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST } ?: return false
        return lightChildren(source, modifiers).any { entry ->
            if (entry.tokenType != KtNodeTypes.ANNOTATION_ENTRY && entry.tokenType != KtNodeTypes.ANNOTATION) return@any false
            val raw = lightText(source, entry)
            val name = raw.removePrefix("@").substringAfter(':').substringBefore('(').trim().substringAfterLast('.')
            name in frameworkInjectedAnnotations ||
                (name == "Suppress" || name == "SuppressWarnings") && unusedMemberSuppressions.any { it in raw }
        }
    }

    // The expression after `=`, null for a delegate or a property with no initializer.
    private fun initializerOf(source: KtSourceElement, node: LighterASTNode): LighterASTNode? {
        val children = significantChildren(source, node)
        val equals = children.indexOfFirst { it.tokenType == KtTokens.EQ }
        return if (equals < 0) null else children.getOrNull(equals + 1)
    }

    // `val TAG = tag()` (or `x.tag()`) is built by a logging helper, so Go keeps it.
    private fun isTagFromTagCall(source: KtSourceElement, node: LighterASTNode, name: String): Boolean {
        if (name != "TAG") return false
        var initializer = initializerOf(source, node) ?: return false
        if (initializer.tokenType == KtNodeTypes.DOT_QUALIFIED_EXPRESSION || initializer.tokenType == KtNodeTypes.SAFE_ACCESS_EXPRESSION) {
            initializer = significantChildren(source, initializer).lastOrNull() ?: return false
        }
        return calleeName(source, initializer) == "tag"
    }

    private fun calleeName(source: KtSourceElement, call: LighterASTNode): String? {
        if (call.tokenType != KtNodeTypes.CALL_EXPRESSION) return null
        val callee = significantChildren(source, call).firstOrNull() ?: return null
        return if (callee.tokenType == KtNodeTypes.REFERENCE_EXPRESSION) lightText(source, callee).trim('`') else null
    }

    // Deleting the property would also delete the effect of a direct initializer that registers or starts something.
    private fun initializerHasSideEffects(source: KtSourceElement, node: LighterASTNode): Boolean {
        val initializer = initializerOf(source, node) ?: return false
        return callsSideEffect(source, initializer)
    }

    private fun callsSideEffect(source: KtSourceElement, node: LighterASTNode): Boolean {
        if (calleeName(source, node)?.let { sideEffectCall.matches(it) } == true) return true
        return lightChildren(source, node).any { callsSideEffect(source, it) }
    }
}
