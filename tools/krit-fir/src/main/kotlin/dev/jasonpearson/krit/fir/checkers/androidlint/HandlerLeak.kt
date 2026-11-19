package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.firstModifierAnchor
import dev.jasonpearson.krit.fir.support.inheritsFrom
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirClassChecker
import org.jetbrains.kotlin.fir.analysis.getChild
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirAnonymousObject
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.utils.isInner
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.symbols.SymbolInternals
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.fir.types.resolvedType
import org.jetbrains.kotlin.lexer.KtTokens

/**
 * Port of the Go HandlerLeak rule (Android Lint `HandlerLeak`), Kotlin side.
 * As in Lint 32.4.1 (HandlerDetector):
 * - an `inner` class that is an `android.os.Handler` is reported on the
 *   declaration's first line (its first annotation or modifier), like Go,
 *   unless one of its own constructors (primary or secondary) takes a
 *   parameter typed `android.os.Looper`, whether or not it reaches the
 *   Handler constructor (Lint takes the Handler to run on the caller's
 *   Looper);
 * - an anonymous object (`object : Handler() { ... }`) that is an
 *   `android.os.Handler` is reported on its `object` keyword, like Go,
 *   wherever it is declared, unless an argument of its superclass
 *   constructor call is a `Looper` (`object : Handler(Looper.getMainLooper())`).
 * The message names the class by its package-qualified name, as Lint does.
 * A nested class that is not `inner`, and a local class, is not reported
 * (Lint skips a class with no containing class).
 *
 * A class or object is a Handler when `android.os.Handler` is in its
 * superclass closure.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`HandlerLeak*.kt`):
 * - With `import android.os.*`, Go treats every supertype name it cannot
 *   match to an explicit import as `android.os.Handler`, so it reports inner
 *   classes and anonymous objects that only implement `Runnable` or extend a
 *   class declared in the file. Those are not Handlers and are not reported here.
 * - Go resolves only the supertype's last name segment through the file's
 *   imports, so it misses a fully qualified `android.os.Handler` supertype, a
 *   type alias, and a subclass of a Handler base class (top-level, nested, or
 *   local). Those are inner or anonymous Handlers and are reported here.
 * - Go resolves the supertype's last name segment through an explicit
 *   `import android.os.Handler` even when a nested class named `Handler`
 *   shadows the import or the supertype is qualified (`Foo.Handler()`).
 *   Those supertypes are not android.os.Handler and are not reported here.
 * - Go reads the Looper parameter's type as written (`Looper` or
 *   `android.os.Looper`), so it misses an import alias or a type alias of
 *   Looper, and reads an anonymous Handler's Looper argument from its text
 *   (it names a looper) or a resolved name. Resolution sees each of them.
 */
internal object HandlerLeak : FirClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "HandlerLeak"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val classCheckers = setOf(HandlerLeak)
    }

    private fun innerMessage(qualifiedName: String) =
        "This `Handler` class should be static or leaks might occur ($qualifiedName)"
    private const val ANONYMOUS_MESSAGE =
        "This `Handler` class should be static or leaks might occur (anonymous android.os.Handler)"

    private val handlerClassId = classIdOf("android.os", "Handler")

    private val looperClassId = classIdOf("android.os", "Looper")

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirClass) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val session = context.session
        when (declaration) {
            is FirAnonymousObject -> {
                if (!isHandler(declaration, session)) return
                if (passesLooperArgument(declaration, session)) return
                report(source.getChild(KtTokens.OBJECT_KEYWORD, depth = 1) ?: source, ANONYMOUS_MESSAGE)
            }
            is FirRegularClass -> {
                if (declaration.classKind != ClassKind.CLASS || !declaration.isInner) return
                if (!isHandler(declaration, session)) return
                if (hasLooperConstructorParameter(declaration, session)) return
                // An inner class always has a modifier list (`inner`); Go
                // reports its first line, where the first annotation or
                // modifier stands.
                report(firstModifierAnchor(source) ?: source, innerMessage(qualifiedName(declaration)))
            }
        }
    }

    // The package and the names of the enclosing classes, as Lint and Go
    // print them; anonymous objects and functions add no name.
    context(context: CheckerContext)
    private fun qualifiedName(declaration: FirRegularClass): String {
        val names = context.containingDeclarations
            .filter { it is FirRegularClassSymbol && it != declaration.symbol }
            .map { (it as FirRegularClassSymbol).name.asString() } + declaration.name.asString()
        @OptIn(SymbolInternals::class)
        val pkg = context.containingDeclarations.filterIsInstance<FirFileSymbol>().firstOrNull()?.fir?.packageDirective?.packageFqName
        return if (pkg == null || pkg.isRoot) names.joinToString(".") else pkg.asString() + "." + names.joinToString(".")
    }

    // Supertypes are read from their own lookup tags, so no class id is
    // resolved from a symbol that may be local.
    private fun isHandler(declaration: FirClass, session: FirSession): Boolean {
        for (ref in declaration.superTypeRefs) {
            val type = ref.coneType.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType ?: continue
            if (type.lookupTag.classId == handlerClassId) return true
            val symbol = type.lookupTag.toClassSymbol(session) ?: continue
            if (inheritsFrom(symbol, session, lookupInterfaces = false) { it == handlerClassId }) return true
        }
        return false
    }

    // Lint's hasLooperConstructorParameter: a constructor of the class itself
    // takes a parameter typed Looper (nullable or not).
    @OptIn(DirectDeclarationsAccess::class)
    private fun hasLooperConstructorParameter(declaration: FirRegularClass, session: FirSession): Boolean =
        declaration.declarations.filterIsInstance<FirConstructor>().any { ctor ->
            ctor.valueParameters.any { isLooper(it.returnTypeRef.coneType, session) }
        }

    // Lint's hasLooperArgument: an argument of the anonymous object's
    // superclass constructor call is a Looper.
    @OptIn(DirectDeclarationsAccess::class)
    private fun passesLooperArgument(declaration: FirAnonymousObject, session: FirSession): Boolean {
        val call = declaration.declarations.filterIsInstance<FirConstructor>().firstOrNull()?.delegatedConstructor
            ?: return false
        return call.argumentList.arguments.any { isLooper(it.resolvedType, session) }
    }

    private fun isLooper(type: ConeKotlinType, session: FirSession): Boolean {
        val expanded = type.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType ?: return false
        return expanded.lookupTag.classId == looperClassId
    }
}
