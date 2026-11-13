package dev.jasonpearson.krit.fir.checkers.style

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.checkers.bodyInCheckedFile
import dev.jasonpearson.krit.fir.checkers.initializerInCheckedFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.firstModifierAnchor
import dev.jasonpearson.krit.fir.support.hasAnnotationNamed
import dev.jasonpearson.krit.fir.support.lightChildren
import org.jetbrains.kotlin.KtFakeSourceElementKind
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.config.LanguageFeature
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.languageVersionSettings
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirPropertyChecker
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.impl.FirDefaultPropertyGetter
import org.jetbrains.kotlin.fir.declarations.utils.isOverride
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.expressions.FirExpression
import org.jetbrains.kotlin.fir.expressions.FirFunctionCall
import org.jetbrains.kotlin.fir.expressions.FirPropertyAccessExpression
import org.jetbrains.kotlin.fir.expressions.FirReturnExpression
import org.jetbrains.kotlin.fir.expressions.FirThisReceiverExpression
import org.jetbrains.kotlin.fir.references.toResolvedCallableSymbol
import org.jetbrains.kotlin.fir.references.toResolvedPropertySymbol
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toRegularClassSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirClassSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirPropertySymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.classId
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.CallableId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

// Flags a private mutable backing property that only exists to expose a read-only
// supertype of itself, which a Kotlin 2.4 explicit backing field expresses in one
// declaration (`val s: StateFlow<Int> field = MutableStateFlow(0)`). Inside the
// declaring class (or the file, for top-level properties) `s` is smart cast to the
// field type, so the writes to the backing property carry over after a rename.
//
// Mirrors the Go ExplicitBackingFieldCandidate rule and reports on the private
// property, on its first modifier like Go (a KDoc belongs to the declaration in
// the light tree, not in Go's node). The pair is:
// - a private, annotation-free `val` with an initializer and no delegate, and
//   a mutable type from a known family: MutableStateFlow, MutableSharedFlow,
//   MutableLiveData, MutableState, the mutable collections and Channel, or a subtype;
// - a non-private, effectively final `val` with no setter, receiver or delegate,
//   whose type is a read-only counterpart of that family (StateFlow, SharedFlow,
//   Flow, LiveData, State, List, Set, Map, Collection, Iterable, ReceiveChannel),
//   whose only value is the backing property: `= _s`, `get() = _s`,
//   `get() { return _s }`, or `_s.asStateFlow()` / `_s.asSharedFlow()`.
// - the backing property is exposed by exactly one such property.
// The backing property may have any number of other uses: inside the declaring
// scope they keep working through the smart cast. Only the language gate decides
// when explicit backing fields exist: the checker asks the compiler session, so a
// module on language version 2.3 or older, or with the feature disabled, is not flagged.
//
// FIR decides on resolution, where Go reads declared types and factory names:
// - a `val s = _s.asStateFlow()` property is flagged without a declared type
//   (Go flags it too but offers no fix), and an `ArrayList` or any subtype of a
//   mutable family type is recognized by its supertypes;
// - a Go lookalike (a project class named `MutableLiveData`) is not flagged.
internal object ExplicitBackingFieldCandidate : FirPropertyChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ExplicitBackingFieldCandidate"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val propertyCheckers = setOf(ExplicitBackingFieldCandidate)
    }

    private val mutableFamily = setOf(
        classIdOf("kotlinx.coroutines.flow", "MutableStateFlow"),
        classIdOf("kotlinx.coroutines.flow", "MutableSharedFlow"),
        classIdOf("androidx.lifecycle", "MutableLiveData"),
        classIdOf("androidx.compose.runtime", "MutableState"),
        classIdOf("kotlin.collections", "MutableList"),
        classIdOf("kotlin.collections", "MutableSet"),
        classIdOf("kotlin.collections", "MutableMap"),
        classIdOf("kotlinx.coroutines.channels", "Channel"),
    )

    private val readOnlyCounterparts = setOf(
        classIdOf("kotlinx.coroutines.flow", "StateFlow"),
        classIdOf("kotlinx.coroutines.flow", "SharedFlow"),
        classIdOf("kotlinx.coroutines.flow", "Flow"),
        classIdOf("androidx.lifecycle", "LiveData"),
        classIdOf("androidx.compose.runtime", "State"),
        classIdOf("kotlin.collections", "List"),
        classIdOf("kotlin.collections", "Set"),
        classIdOf("kotlin.collections", "Map"),
        classIdOf("kotlin.collections", "Collection"),
        classIdOf("kotlin.collections", "Iterable"),
        classIdOf("kotlinx.coroutines.channels", "ReceiveChannel"),
    )

    private val wrappers = setOf(
        CallableId(FqName("kotlinx.coroutines.flow"), Name.identifier("asStateFlow")),
        CallableId(FqName("kotlinx.coroutines.flow"), Name.identifier("asSharedFlow")),
    )

    private val jvmField = setOf("JvmField")


    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirProperty) {
        if (declaration.isLocal) return
        val session = context.session
        if (!session.languageVersionSettings.supportsFeature(LanguageFeature.ExplicitBackingFields)) return
        val backing = backingOf(declaration, session) ?: return
        // Another property exposing the same backing property cannot both become its field.
        if (siblingExposures(declaration, backing, session) != 1) return
        val anchor = backing.source?.let { firstModifierAnchor(it) } ?: backing.source ?: return
        report(
            anchor,
            "Backing property '${backing.name.asString()}' only exposes '${declaration.name.asString()}'; " +
                "declare '${declaration.name.asString()}' with an explicit backing field (`field = ...`) " +
                "and remove '${backing.name.asString()}'.",
        )
    }

    // An override is open unless its class is final, so a final class makes it effectively final.
    context(context: CheckerContext)
    private fun isEffectivelyFinal(declaration: FirProperty): Boolean {
        val status = declaration.status
        if (status.modality == Modality.FINAL) return true
        if (!declaration.isOverride || status.modality != Modality.OPEN) return false
        val owner = context.containingDeclarations.lastOrNull { it is FirClassSymbol<*> } as? FirClassSymbol<*> ?: return false
        return owner.resolvedStatus.modality == Modality.FINAL && !hasOpenKeyword(declaration.source)
    }

    private fun hasOpenKeyword(source: KtSourceElement?): Boolean {
        if (source == null) return false
        val modifiers = lightChildren(source, source.lighterASTNode).firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST }
            ?: return false
        return lightChildren(source, modifiers).any { it.tokenType == KtTokens.OPEN_KEYWORD }
    }

    // The private property that [declaration] only exposes, when [declaration] is a candidate exposure.
    context(context: CheckerContext)
    private fun backingOf(declaration: FirProperty, session: FirSession): FirPropertySymbol? {
        if (declaration.source?.kind is KtFakeSourceElementKind) return null
        if (declaration.isVar || declaration.delegate != null || declaration.setter != null) return null
        if (declaration.receiverParameter != null || declaration.contextParameters.isNotEmpty()) return null
        if (declaration.status.isExpect || !isEffectivelyFinal(declaration)) return null
        if (declaration.visibility == Visibilities.Private || declaration.visibility == Visibilities.PrivateToThis) return null
        if (declaration.hasAnnotationNamed(jvmField)) return null
        val getter = declaration.getter?.takeIf { it !is FirDefaultPropertyGetter }
        val expression: FirExpression? = when {
            getter == null -> declaration.initializer
            declaration.initializer == null -> (getter.body?.statements?.singleOrNull() as? FirReturnExpression)?.result
            else -> null
        }
        return exposedBacking(expression, declaration.symbol, session)
    }

    // The backing property when [expression] is `_s`, `this._s`, `_s.asStateFlow()` or `_s.asSharedFlow()`,
    // the property is a valid backing property for [exposing], and the types fit.
    private fun exposedBacking(expression: FirExpression?, exposing: FirPropertySymbol, session: FirSession): FirPropertySymbol? {
        var access = expression
        if (access is FirFunctionCall) {
            if (access.calleeReference.toResolvedCallableSymbol()?.callableId !in wrappers) return null
            access = access.extensionReceiver
        }
        if (access !is FirPropertyAccessExpression) return null
        val receiver = access.explicitReceiver
        if (receiver != null && receiver !is FirThisReceiverExpression) return null
        val backing = access.calleeReference.toResolvedPropertySymbol() ?: return null
        // A primary-constructor `val` is a parameter, which cannot become an explicit backing field.
        if (backing.source?.kind is KtFakeSourceElementKind) return null
        if (backing == exposing || !backing.isVal || backing.hasDelegate || !backing.hasInitializer) return null
        if (backing.resolvedStatus.visibility != Visibilities.Private) return null
        if (backing.callableId?.classId != exposing.callableId?.classId) return null
        if (backing.resolvedAnnotationsWithClassIds.isNotEmpty()) return null
        if (!isMutableFamily(backing.resolvedReturnType, session)) return null
        val exposedType = exposing.resolvedReturnType.fullyExpandedType(session) as? ConeClassLikeType ?: return null
        if (exposedType.isMarkedNullable || exposedType.classId !in readOnlyCounterparts) return null
        return backing
    }

    private fun isMutableFamily(type: ConeKotlinType, session: FirSession): Boolean {
        val expanded = type.fullyExpandedType(session) as? ConeClassLikeType ?: return false
        if (expanded.isMarkedNullable) return false
        if (expanded.classId in mutableFamily) return true
        val symbol = expanded.lookupTag.toRegularClassSymbol(session) ?: return false
        return lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
            .any { it.classId in mutableFamily }
    }

    // How many properties of the declaring class (or file) expose [backing] the way [declaration] does.
    @OptIn(DirectDeclarationsAccess::class, SymbolInternals::class)
    context(context: CheckerContext)
    private fun siblingExposures(declaration: FirProperty, backing: FirPropertySymbol, session: FirSession): Int {
        val owner = context.containingDeclarations.lastOrNull { it is FirClassSymbol<*> }
        val siblings: List<FirPropertySymbol> = if (owner is FirClassSymbol<*>) {
            owner.declarationSymbols.filterIsInstance<FirPropertySymbol>()
        } else {
            // A top-level property lies in the file under check (the private backing property is file-private).
            context.containingFileSymbol?.fir?.declarations.orEmpty().filterIsInstance<FirProperty>().map { it.symbol }
        }
        return siblings.count { sibling ->
            sibling == declaration.symbol || sibling.exposes(backing, session)
        }
    }

    // True when this property, declared in the file under check, only exposes [backing].
    context(context: CheckerContext)
    private fun FirPropertySymbol.exposes(backing: FirPropertySymbol, session: FirSession): Boolean {
        if (isVar || hasDelegate || resolvedStatus.visibility == Visibilities.Private) return false
        val expression = if (getterSymbol?.isDefault == false) {
            (getterSymbol?.bodyInCheckedFile()?.statements?.singleOrNull() as? FirReturnExpression)?.result
        } else {
            initializerInCheckedFile()
        }
        return exposedBacking(expression, this, session) == backing
    }
}
