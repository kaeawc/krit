package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.getChild
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Port of the Go ObjectExtendsThrowable rule: a named `object` declaration
 * (top-level, nested, or `data object`) that is a `Throwable`, reported on the
 * declaration's first line (its modifier list, else `object`), like Go. The
 * message names the object and the superclass it extends directly.
 *
 * The object is a Throwable when `kotlin.Throwable` (or `java.lang.Throwable`)
 * is in its superclass closure, so a direct `Throwable`, `Exception`, `Error`,
 * or `RuntimeException` counts (as in Go), and so do their subclasses, user
 * base classes extending them, type aliases, and import aliases.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`ObjectExtendsThrowable*.kt`):
 * - Go takes every type name written anywhere in the object's delegation
 *   specifiers, so it reports an object that only mentions `Exception` or
 *   `Error` in a type argument (`Comparator<Exception>`), a constructor
 *   argument (`Base(Exception::class)`, `Base(listOf<Error>())`), or a lambda
 *   parameter type, and an object extending a lookalike class named
 *   `Exception`, `Error`, `Throwable`, or `RuntimeException` that is not a
 *   Throwable. None of those objects is a Throwable, so none is reported here.
 *   The same holds for type names inside an interface delegate expression
 *   (`Sink by object : Sink { ... Throwable ... }`).
 * - Go's resolver never indexes an object declared in a companion object or
 *   enum class body (or nested in one), so Go searches that object's whole
 *   text for `: Exception`, `: Error`, `: IllegalStateException(`, and the
 *   like. It then reports an object whose member type annotation
 *   (`val cause: Exception?`), interface name (`: ErrorHandler`), or nested
 *   object mentions one. Those objects are not Throwables and are not
 *   reported here; the Throwable objects Go finds this way still are.
 * - Go looks the object up by simple name, so a class with the same name
 *   declared later in the file replaces it: Go then reads that class's
 *   supertypes, reporting an object that is not a Throwable and missing one
 *   that is. Here each object is judged by its own supertypes.
 * - Go matches only the four direct names above, so it misses an object
 *   extending any other Throwable (`IllegalStateException`, a user exception
 *   base class, a type alias, an import alias), and it never visits a
 *   companion object. Those are singletons that are Throwables, so they are
 *   reported here.
 */
internal object ObjectExtendsThrowable :
    FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ObjectExtendsThrowable"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(ObjectExtendsThrowable)
    }

    private val throwableClassIds = setOf(
        StandardClassIds.Throwable,
        ClassId(FqName("java.lang"), Name.identifier("Throwable")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirRegularClass) {
        if (declaration.classKind != ClassKind.OBJECT) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val superclass = throwableSuperclass(declaration, context.session) ?: return
        // K2 places a diagnostic on an object at its `object` keyword; Go
        // reports the declaration's first line, where its modifier list
        // (annotations included) starts.
        report(
            source.getChild(KtNodeTypes.MODIFIER_LIST, depth = 1) ?: source,
            "Object '${declaration.name.asString()}' extends '$superclass'. " +
                "Objects that extend Throwable are singletons and lose stack trace information.",
        )
    }

    // The simple name of the direct supertype that makes the object a
    // Throwable, or null when it is not one. Supertypes are read from their
    // own lookup tags, so no class id is resolved from a symbol that may be
    // local.
    private fun throwableSuperclass(declaration: FirRegularClass, session: FirSession): String? {
        for (ref in declaration.superTypeRefs) {
            val type = ref.coneType.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType ?: continue
            if (type.lookupTag.classId in throwableClassIds) return type.lookupTag.name.asString()
            val symbol = type.lookupTag.toClassSymbol(session) ?: continue
            val isThrowable = lookupSuperTypes(symbol, lookupInterfaces = false, deep = true, useSiteSession = session)
                .any { it.lookupTag.classId in throwableClassIds }
            if (isThrowable) return type.lookupTag.name.asString()
        }
        return null
    }
}
