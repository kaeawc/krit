package dev.jasonpearson.krit.fir.checkers.androidlint

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.hasAnnotationNamed
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.descriptors.Visibilities
import org.jetbrains.kotlin.descriptors.Visibility
import org.jetbrains.kotlin.descriptors.Modality
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirAnonymousObjectChecker
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.analysis.getChild
import org.jetbrains.kotlin.fir.declarations.DirectDeclarationsAccess
import org.jetbrains.kotlin.fir.declarations.FirAnonymousObject
import org.jetbrains.kotlin.fir.declarations.FirClass
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.hasAnnotation
import org.jetbrains.kotlin.fir.declarations.utils.isInner
import org.jetbrains.kotlin.fir.declarations.utils.modality
import org.jetbrains.kotlin.fir.declarations.utils.visibility
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go FragmentConstructor rule (Android Lint `ValidFragment`): a
 * non-abstract, non-sealed Fragment subclass the framework cannot
 * re-instantiate, reported on the class declaration's first line (its
 * modifier list, else `class`), like Go. The cases, as in Android Lint 32.4.1:
 * - the class is `private` or `protected`, which are not public JVM classes
 *   (Kotlin `internal` compiles to a public class and is accepted);
 * - the class is `inner`, so it needs an outer instance;
 * - the no-arg constructor is `private` or `protected`
 *   (reported on the constructor);
 * - the class declares a constructor with parameters and has no constructor
 *   callable without arguments;
 * - the class is an anonymous object.
 *
 * As in Go, a constructor counts as no-arg when it has no parameters, or when
 * it is the primary constructor and every parameter has a default value (the
 * compiler then emits a no-arg JVM constructor). A secondary constructor
 * whose parameters all have defaults gets no such JVM constructor and does
 * not count, which is also Go's verdict: Go never sees a default on a
 * secondary constructor parameter.
 *
 * The class is a Fragment when `android.app.Fragment`,
 * `androidx.fragment.app.Fragment`, or the support library's
 * `android.support.v4.app.Fragment` is in its superclass closure. That covers
 * the direct supertypes Go names (`Fragment`, `DialogFragment`,
 * `ListFragment`, `PreferenceFragment`, `PreferenceFragmentCompat`,
 * `BottomSheetDialogFragment`), all of which extend one of those roots.
 *
 * A class with a constructor annotated `@Inject` or `@AssistedInject`
 * (primary or secondary) is not reported: dependency injection builds it
 * through a custom FragmentFactory, as in Go.
 *
 * Fragments in test source files are skipped, as in Go. Go also skips a
 * Fragment built in the same file as `launchFragment(MyFragment(args))`;
 * FIR does not look at call sites.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`FragmentConstructor*.kt`):
 * - Go matches the direct supertype's simple name, so it reports a class
 *   extending a lookalike named `Fragment` (or another listed name) that is
 *   not an Android Fragment. The framework never re-instantiates those, so
 *   they are not reported here.
 * - Go misses a Fragment subclass whose Fragment supertype is indirect (a
 *   user base class), a type alias, or an import alias. Those are Fragments
 *   without a no-arg constructor, so they are reported here.
 * - Go starts from "has a no-arg constructor" and only a primary constructor
 *   with parameters clears it, so it misses a class with no primary
 *   constructor whose secondary constructors all take parameters. That class
 *   has no no-arg constructor, so it is reported here.
 * - Go reports a class whose only no-arg-callable constructor is a secondary
 *   constructor annotated `@JvmOverloads` with every parameter defaulted.
 *   The compiler emits a no-arg JVM overload for it, so the class has the
 *   constructor the framework needs and is not reported here.
 * - Go collects secondary constructors from the whole class body, nested and
 *   local classes included, so a no-arg constructor of a nested class hides a
 *   finding on the outer Fragment. Only the class's own constructors count
 *   here.
 */
internal object FragmentConstructor :
    FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "FragmentConstructor"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(FragmentConstructor)
        override val anonymousObjectCheckers = setOf(AnonymousFragment)
    }

    private const val MESSAGE =
        "Fragment subclass must have a default (no-arg) constructor for framework re-instantiation."
    private const val ANONYMOUS_MESSAGE =
        "Fragments should be static such that they can be re-instantiated by the system, and anonymous classes are not static."

    private val fragmentClassIds = setOf(
        ClassId(FqName("android.app"), Name.identifier("Fragment")),
        ClassId(FqName("androidx.fragment.app"), Name.identifier("Fragment")),
        ClassId(FqName("android.support.v4.app"), Name.identifier("Fragment")),
    )

    private val injectAnnotationNames = setOf("Inject", "AssistedInject")

    private val jvmOverloadsClassId = ClassId(FqName("kotlin.jvm"), Name.identifier("JvmOverloads"))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirRegularClass) {
        if (declaration.classKind != ClassKind.CLASS) return
        if (declaration.modality == Modality.ABSTRACT || declaration.modality == Modality.SEALED) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (!isFragment(declaration, context.session)) return
        // Test fragments are built by the test, not re-instantiated by the framework, as in Go.
        if (isInTestFile()) return
        @OptIn(DirectDeclarationsAccess::class)
        val constructors = declaration.declarations.filterIsInstance<FirConstructor>()
        if (constructors.isEmpty()) return
        // A DI-annotated constructor means a custom FragmentFactory builds the class.
        if (constructors.any { isInjectAnnotated(it) }) return
        // Go reports the declaration's first line, where its modifier list
        // (annotations included) starts, else the `class` keyword.
        val anchor = source.getChild(KtNodeTypes.MODIFIER_LIST, depth = 1)
            ?: source.getChild(KtTokens.CLASS_KEYWORD, depth = 1)
            ?: source
        if (declaration.visibility.isNonPublicJvm()) {
            report(anchor, "This fragment class should be public (${declaration.name.asString()}).")
            return
        }
        if (declaration.isInner) {
            report(anchor, "This fragment inner class should be static (${declaration.name.asString()}).")
            return
        }
        val hiddenNoArg = constructors.firstOrNull { it.valueParameters.isEmpty() && it.visibility.isNonPublicJvm() }
        if (hiddenNoArg != null) {
            report(hiddenNoArg.source ?: anchor, "The default constructor of a fragment must be public.")
            return
        }
        if (constructors.any { isNoArg(it, context.session) }) return
        report(anchor, MESSAGE)
    }

    // Private and protected members are not public in the class file. Kotlin
    // internal members compile to public ones, so they are accepted.
    private fun Visibility.isNonPublicJvm(): Boolean = this == Visibilities.Private || this == Visibilities.Protected

    // A constructor the framework can call without arguments: one with no
    // parameters, or one whose parameters all have defaults when the compiler
    // also emits a no-arg JVM overload for it (the primary constructor, or any
    // constructor annotated `@JvmOverloads`). A secondary constructor whose
    // parameters all have defaults gets no such overload, so, as in Go, it
    // does not count.
    private fun isNoArg(ctor: FirConstructor, session: FirSession): Boolean {
        val params = ctor.valueParameters
        if (params.isEmpty()) return true
        if (params.any { it.defaultValue == null }) return false
        return ctor.isPrimary || ctor.hasAnnotation(jvmOverloadsClassId, session)
    }

    private fun isInjectAnnotated(ctor: FirConstructor): Boolean = ctor.hasAnnotationNamed(injectAnnotationNames)

    // Supertypes are read from their own lookup tags, so no class id is
    // resolved from a symbol that may be local.
    private fun isFragment(declaration: FirClass, session: FirSession): Boolean {
        for (ref in declaration.superTypeRefs) {
            val type = ref.coneType.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType ?: continue
            if (type.lookupTag.classId in fragmentClassIds) return true
            val symbol = type.lookupTag.toClassSymbol(session) ?: continue
            val extendsFragment = lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = session)
                .any { it.lookupTag.classId in fragmentClassIds }
            if (extendsFragment) return true
        }
        return false
    }

    /** Reports `object : Fragment() {}`, which has no name to re-instantiate. */
    private object AnonymousFragment : FirAnonymousObjectChecker(MppCheckerKind.Common) {
        context(context: CheckerContext, reporter: DiagnosticReporter)
        override fun check(declaration: FirAnonymousObject) {
            val source = declaration.source ?: return
            if (source.kind !is KtRealSourceElementKind) return
            if (isInTestFile()) return
            if (!isFragment(declaration, context.session)) return
            report(source, ANONYMOUS_MESSAGE)
        }
    }
}
