package dev.jasonpearson.krit.fir.checkers.coroutines

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toRegularClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeDefinitelyNotNullType
import org.jetbrains.kotlin.fir.types.ConeIntersectionType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.type
import org.jetbrains.kotlin.fir.types.upperBoundIfFlexible
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go StateFlowMutableLeak rule: a publicly visible,
 * non-override property whose type exposes kotlinx.coroutines.flow.MutableStateFlow
 * (directly, nullable, through a type alias, as a subtype such as a class or
 * interface extending it or a type parameter bounded by it, or as a type
 * argument such as `List<MutableStateFlow<T>>`), reported on the property's
 * first line (its modifier list, or `val`/`var` when it has none), like Go.
 *
 * Covered like Go: class, object, companion, interface, and top-level
 * properties, explicit or inferred MutableStateFlow types, delegated and
 * getter-backed properties, `private set` (the getter stays public), and
 * properties of non-public classes (Go checks only the property's own
 * modifiers). Skipped like Go: private/protected/internal and override
 * properties, primary-constructor `val` parameters, anything declared inside
 * a function, getter, or setter body, and test source files.
 *
 * Deliberate precision differences from Go's substring match on the property
 * text (see the golden data): a property whose declared type is read-only is
 * not reported just because its initializer mentions MutableStateFlow
 * (`val s: StateFlow<Int> = MutableStateFlow(0)`,
 * `val s = MutableStateFlow(0).asStateFlow()`), nor because only its name,
 * a comment, a string, or an unrelated class name contains the word; locals in
 * an init block, a secondary-constructor body (tree-sitter parses it as a
 * block, not a function_body, so Go reports them), or a property-initializer
 * lambda, and members of anonymous objects, enum-entry bodies, or local
 * classes are not exposed; and a MutableStateFlow type the source does not
 * spell out (an inferred call result, an import alias, a type alias, a subtype
 * whose name lacks the word) is reported.
 */
internal object StateFlowMutableLeak : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "StateFlowMutableLeak"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(StateFlowMutableLeak)
    }

    private val mutableStateFlow = ClassId(FqName("kotlinx.coroutines.flow"), Name.identifier("MutableStateFlow"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        if (declaration.isLocal) return
        val source = declaration.source ?: return
        // Primary-constructor `val`/`var` parameters are class parameters in Go,
        // not property declarations.
        if (source.kind is KtFakeSourceElementKind) return
        if (!isExposedContainer(declaration)) return
        if (declaration.isOverride) return
        val visibility = declaration.visibility
        if (visibility == Visibilities.Private || visibility == Visibilities.Protected ||
            visibility == Visibilities.Internal || visibility == Visibilities.PrivateToThis
        ) return
        if (!mentionsMutableStateFlow(declaration.returnTypeRef.coneType, context.session, depth = 0)) return
        // Go skips scanner.IsTestFile files; the check request carries that
        // classification (configured test paths included).
        if (isInTestFile()) return

        report(source, "MutableStateFlow '${declaration.name.asString()}' is publicly exposed. Keep it private and expose as StateFlow<T>.")
    }

    // Top-level properties and members of named, non-local classes and objects.
    // Anything nested in a function, accessor, constructor, init block, lambda,
    // anonymous object, or local class is not part of the declaring API.
    context(context: CheckerContext)
    private fun isExposedContainer(declaration: FirProperty): Boolean =
        context.containingDeclarations.all { symbol ->
            symbol == declaration.symbol ||
                symbol is FirFileSymbol ||
                (symbol is FirRegularClassSymbol && symbol.resolvedStatus.visibility != Visibilities.Local)
        }

    // True when the type is MutableStateFlow or a subtype of it (a class or
    // interface that extends it, or a type parameter bounded by it), or carries
    // one as a type argument (`List<MutableStateFlow<T>>`).
    private fun mentionsMutableStateFlow(type: ConeKotlinType, session: FirSession, depth: Int): Boolean {
        if (depth > MAX_TYPE_DEPTH) return false
        val expanded = type.fullyExpandedType(session)
        val bounds = listOf(expanded.lowerBoundIfFlexible(), expanded.upperBoundIfFlexible()).distinct()
        return bounds.any { bound ->
            when (bound) {
                is ConeDefinitelyNotNullType -> mentionsMutableStateFlow(bound.original, session, depth + 1)
                is ConeIntersectionType -> bound.intersectedTypes.any { mentionsMutableStateFlow(it, session, depth + 1) }
                is ConeTypeParameterType -> bound.lookupTag.typeParameterSymbol.resolvedBounds.any {
                    mentionsMutableStateFlow(it.coneType, session, depth + 1)
                }
                is ConeClassLikeType -> isMutableStateFlowClass(bound, session) ||
                    bound.typeArguments.any { argument ->
                        val argumentType = argument.type ?: return@any false
                        mentionsMutableStateFlow(argumentType, session, depth + 1)
                    }
                else -> false
            }
        }
    }

    private fun isMutableStateFlowClass(type: ConeClassLikeType, session: FirSession): Boolean {
        if (type.classId == mutableStateFlow) return true
        val symbol = type.lookupTag.toRegularClassSymbol(session) ?: return false
        return lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
            .any { it.classId == mutableStateFlow }
    }

    private const val MAX_TYPE_DEPTH = 16
}
