package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.firstModifierAnchor
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import dev.jasonpearson.krit.fir.support.simpleName
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirElement
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirFileChecker
import org.jetbrains.kotlin.fir.declarations.FirFile
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.utils.isLateInit
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.expressions.FirCallableReferenceAccess
import org.jetbrains.kotlin.fir.expressions.FirDesugaredAssignmentValueReferenceExpression
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirQualifiedAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirVariableAssignment
import org.jetbrains.kotlin.fir.references.FirBackingFieldReference
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.unwrapFakeOverrides
import org.jetbrains.kotlin.fir.visitors.FirVisitorVoid
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.lexer.KtTokens

// Flags a `var` that is never reassigned, with the Go rule's scope and message
// (`Variable 'x' could be val.`, on the declaration's first line).
//
// Go collects assignment targets by name inside the enclosing block or class and has to guess when a
// name is written through another receiver, shadowed by a parameter or reached from a companion. This
// checker reads resolution instead: a `var` is reassigned when some assignment, `++`/`--` or
// compound assignment in the file resolves to its symbol, or a callable reference points at it (which
// can write it). A same-named variable in another scope is a different symbol, so shadowing needs no
// special case.
//
// Covered like Go: a local `var` and a `private` member or top-level `var` of a declaration in a
// `.kt` file. A private property is visible only in its file, so the file holds every write. Not
// reported, like Go: `override`, `lateinit` when `ignoreLateinitVar` is set, a delegated property, a
// property with an explicit setter, a framework-injected property (`@Inject`, `@Mock`, ...) or one
// marked `@Volatile`/`@Suppress("unused")`, constructor `var` parameters, destructuring entries and
// test files.
//
// Deliberate difference from Go: a `field++` or `field = ...` in a getter counts as a write. The
// reported finding has no auto-fix; when Go also reports the line, its fix is kept.
internal object VarCouldBeVal : FirFileChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "VarCouldBeVal"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val fileCheckers = setOf(VarCouldBeVal)
    }

    private class Candidate(val name: String, val symbol: FirPropertySymbol, val anchor: KtSourceElement)

    // Annotations after which a property is initialised by a framework (Go's frameworkAnnotationNames).
    private val frameworkAnnotations = setOf(
        "Mock", "MockK", "RelaxedMockK", "SpyK", "InjectMocks", "Inject", "Autowired", "BindView", "BindViews",
        "Captor", "Spy", "Value", "EJB", "Resource", "PersistenceContext",
    )

    // Go skips a property that suppresses the unused-member family, since it cannot be judged from here.
    private val suppressedUnused = listOf(
        "\"unused\"", "\"UNUSED_PARAMETER\"", "\"UNUSED_VARIABLE\"", "\"UnusedPrivateProperty\"",
        "\"UnusedPrivateMember\"", "\"UnusedPrivateFunction\"", "\"UnusedVariable\"",
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirFile) {
        if (isInTestFile()) return
        val ignoreLateinit = config()["ignoreLateinitVar"] as? Boolean ?: false
        val candidates = ArrayList<Candidate>()
        val written = HashSet<FirBasedSymbol<*>>()
        declaration.accept(object : FirVisitorVoid() {
            override fun visitElement(element: FirElement) {
                when (element) {
                    is FirProperty -> candidate(element, ignoreLateinit)?.let { candidates += it }
                    is FirVariableAssignment -> writtenSymbol(element.lValue)?.let { written += it }
                    is FirCallableReferenceAccess -> element.calleeReference.toResolvedCallableSymbol()
                        ?.unwrapFakeOverrides()?.let { written += it }
                }
                element.acceptChildren(this)
            }
        })
        for (candidate in candidates) {
            if (candidate.symbol !in written) {
                report(candidate.anchor, "Variable '${candidate.name}' could be val.")
            }
        }
    }

    // The property a write goes to: the variable itself, or the backing field a getter or initializer writes.
    private fun writtenSymbol(target: FirExpression): FirBasedSymbol<*>? {
        // `+=`, `++` and `--` write through a desugared reference to the original access.
        val unwrapped = if (target is FirDesugaredAssignmentValueReferenceExpression) target.expressionRef.value else target
        val access = unwrapped as? FirQualifiedAccessExpression ?: return null
        val reference = access.calleeReference
        if (reference is FirBackingFieldReference) return reference.resolvedSymbol.propertySymbol
        return reference.toResolvedCallableSymbol()?.unwrapFakeOverrides()
    }

    private fun candidate(property: FirProperty, ignoreLateinit: Boolean): Candidate? {
        if (!property.isVar) return null
        val source = property.source ?: return null
        if (source.kind !is KtRealSourceElementKind || source.elementType != KtNodeTypes.PROPERTY) return null
        // Go reports a local, or a member or top-level property that is private.
        if (!property.isLocal && property.visibility != Visibilities.Private) return null
        if (property.isOverride || property.delegate != null) return null
        if (ignoreLateinit && property.isLateInit) return null
        if (hasExplicitSetter(property)) return null
        if (hasExemptAnnotation(property)) return null
        val anchor = anchorOf(source) ?: return null
        return Candidate(property.name.asString(), property.symbol, anchor)
    }

    private fun hasExplicitSetter(property: FirProperty): Boolean =
        property.setter?.source?.kind is KtRealSourceElementKind

    private fun hasExemptAnnotation(property: FirProperty): Boolean {
        val annotations = property.annotations + property.backingField?.annotations.orEmpty() +
            property.getter?.annotations.orEmpty()
        return annotations.any { annotation ->
            val name = annotation.simpleName()
            name in frameworkAnnotations || name == "Volatile" ||
                (name == "Suppress" || name == "SuppressWarnings") &&
                annotation.source?.let { s -> s.treeStructure.toString(s.lighterASTNode).toString() }
                    ?.let { text -> suppressedUnused.any { it in text } } == true
        }
    }

    // Go reports a declaration on its first line: where the modifier list starts, else the `var` keyword.
    // A preceding KDoc belongs to the declaration in the light tree but not in Go's node.
    private fun anchorOf(source: KtSourceElement): KtSourceElement? {
        firstModifierAnchor(source)?.let { return it }
        val keyword = lightChildren(source, source.lighterASTNode).firstOrNull { it.tokenType == KtTokens.VAR_KEYWORD }
        return keyword?.let { lightSourceOf(it, source) }
    }
}
