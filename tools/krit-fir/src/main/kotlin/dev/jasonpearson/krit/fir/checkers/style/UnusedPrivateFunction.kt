package dev.jasonpearson.krit.fir.checkers.style

import com.intellij.lang.LighterASTNode
import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.goFullMatchRegex
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.lightText
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
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.references.FirResolvedNamedReference
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirCallableSymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.lexer.KtTokens

// Flags a private function that nothing in its file calls, with the Go rule's scope and message
// (`Private function `f` is unused.`, on the function's name).
//
// Go looks for the name anywhere else in the file, so a same-named method of another class, an overload,
// a comment or a string keeps it quiet. This checker reads resolution instead: the function is used when
// a resolved reference in the file (a call, a callable reference, an operator or delegate convention, an
// `invoke`) points at its symbol. A private function is visible only in its file, so the file holds every
// use. A call from the function's own body does not count, like Go.
//
// Not reported, like Go: test files, a name matching the `allowedNames` option (a full match), and a
// function that carries an entry-point annotation (`@Preview`, `@Test`, `@Provides`, `@JvmStatic`, ...) or
// any annotation whose name contains `Preview`.
internal object UnusedPrivateFunction : FirFileChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "UnusedPrivateFunction"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(UnusedPrivateFunction)
    }

    // Go's `entryPointAnnotationNames`: frameworks and tooling call these functions without a source reference.
    private val entryPointAnnotations = setOf(
        "Preview", "ComposePreview", "PreviewParameter", "PreviewLightDark", "DarkPreview", "LightPreview",
        "NightPreview", "DayPreview", "Test", "ParameterizedTest", "BeforeEach", "AfterEach", "BeforeAll",
        "AfterAll", "Before", "After", "BeforeClass", "AfterClass", "ParameterizedRobolectricTestRunner.Parameters",
        "Parameters", "Provides", "Binds", "BindsInstance", "Module", "JvmStatic", "JvmName", "JvmField", "Keep",
        "UsedByReflection", "UsedByNative", "VisibleForTesting", "SerializedName", "JsonCreator", "JsonProperty",
    )

    private class Candidate(val name: String, val symbol: FirBasedSymbol<*>, val anchor: KtSourceElement)

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        if (isInTestFile()) return
        val allowed = allowedNames()
        val candidates = ArrayList<Candidate>()
        val referenced = HashSet<FirBasedSymbol<*>>()
        // The functions whose own text is being walked: Go ignores a name inside the declaration itself.
        val inside = HashSet<FirBasedSymbol<*>>()
        declaration.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                when (element) {
                    is FirResolvedNamedReference -> addReference(element.resolvedSymbol, inside, referenced)
                    is FirNamedFunction -> {
                        val candidate = candidateOf(element, allowed)
                        if (candidate != null) {
                            candidates += candidate
                            inside += candidate.symbol
                            element.acceptChildren(this)
                            inside -= candidate.symbol
                            return
                        }
                    }
                }
                element.acceptChildren(this)
            }
        })
        for (candidate in candidates) {
            if (candidate.symbol !in referenced) report(candidate.anchor, "Private function `${candidate.name}` is unused.")
        }
    }

    private fun addReference(symbol: FirBasedSymbol<*>, inside: Set<FirBasedSymbol<*>>, referenced: MutableSet<FirBasedSymbol<*>>) {
        if (symbol in inside) return
        // A member called through a substituted receiver resolves to a fake override of the declaration.
        val declared = if (symbol is FirCallableSymbol<*>) symbol.unwrapFakeOverrides() else symbol
        if (declared in inside) return
        referenced += symbol
        referenced += declared
    }

    // Go leaves the field unset for an empty or invalid pattern, so nothing is allowed then.
    private fun allowedNames(): Regex? = (config()["allowedNames"] as? String)?.takeIf { it.isNotEmpty() }?.let(::goFullMatchRegex)

    private fun candidateOf(declaration: FirNamedFunction, allowed: Regex?): Candidate? {
        // A local function has local visibility, so an explicit `private` is what makes a function a candidate.
        if (declaration.visibility != Visibilities.Private) return null
        val source = declaration.source ?: return null
        if (source.kind !is KtRealSourceElementKind || source.elementType != KtNodeTypes.FUN) return null
        val node = source.lighterASTNode
        val children = lightChildren(source, node)
        val identifier = children.firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        // Go's message keeps the name as written, backticks included.
        val written = lightText(source, identifier)
        val name = written.trim('`')
        if (name.isEmpty() || allowed?.matches(name) == true || hasEntryPointAnnotation(source, children)) return null
        return Candidate(written, declaration.symbol, lightSourceOf(identifier, source))
    }

    // Go reads the annotation text before `fun`: the simple name, past a use-site target and any arguments.
    private fun hasEntryPointAnnotation(source: KtSourceElement, children: List<LighterASTNode>): Boolean {
        val modifiers = children.firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST } ?: return false
        return lightChildren(source, modifiers).any { entry ->
            if (entry.tokenType != KtNodeTypes.ANNOTATION_ENTRY && entry.tokenType != KtNodeTypes.ANNOTATION) return@any false
            val name = lightText(source, entry).removePrefix("@").substringAfter(':').substringBefore('(').trim().substringAfterLast('.')
            name in entryPointAnnotations || "Preview" in name
        }
    }
}
