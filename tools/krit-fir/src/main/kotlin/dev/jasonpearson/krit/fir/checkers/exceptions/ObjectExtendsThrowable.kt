package dev.jasonpearson.krit.fir.checkers.exceptions

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.inheritsFromAny
import dev.jasonpearson.krit.fir.support.modifierKeywordAnchor
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.descriptors.ClassKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.analysis.getChild
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.StandardClassIds

/**
 * Port of the Go ObjectExtendsThrowable rule: an `object` declaration
 * (top-level, nested, companion, or `data object`) that is a `Throwable`,
 * reported on its name (an unnamed companion on `companion`), like Go, with
 * Go's message.
 *
 * The object is a Throwable when `kotlin.Throwable` (or `java.lang.Throwable`)
 * is in its superclass closure, so a direct `Throwable`, `Exception`, `Error`,
 * or `RuntimeException` counts (as in Go), and so do their subclasses, user
 * base classes extending them, type aliases, and import aliases.
 *
 * Go reads the superclass from the source: the delegation specifier that calls
 * a constructor, matched by simple name against `Throwable`, `Exception`,
 * `Error`, `RuntimeException`, `IllegalStateException`,
 * `IllegalArgumentException`, `IOException` and
 * `UnsupportedOperationException`, unless the file declares or imports a
 * project class with that name. Interfaces, type arguments, constructor
 * arguments and `by` delegates never count, in Go or here.
 *
 * Deliberate difference from Go, pinned in `ObjectExtendsThrowableRecall.kt`:
 * Go misses an object extending any other Throwable (`StackOverflowError`, a
 * user exception base class, a type alias, an import alias). Those are
 * singletons that are Throwables, so they are reported here.
 */
internal object ObjectExtendsThrowable :
    FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ObjectExtendsThrowable"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(ObjectExtendsThrowable)
    }

    private val throwableClassIds = setOf(
        StandardClassIds.Throwable,
        classIdOf("java.lang", "Throwable"),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirRegularClass) {
        if (declaration.classKind != ClassKind.OBJECT) return
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        if (!extendsThrowable(declaration, context.session)) return
        // Go reports the object's name, or the `companion` keyword of an
        // unnamed companion, so annotations and modifiers above it do not move
        // the line.
        report(
            source.getChild(KtTokens.IDENTIFIER, depth = 1)
                ?: modifierKeywordAnchor(source, KtTokens.COMPANION_KEYWORD)
                ?: source.getChild(KtTokens.OBJECT_KEYWORD, depth = 1)
                ?: source,
            "An `object` should not extend and type of Throwable. Throwables are stateful and should be " +
                "instantiated only when needed for when a specific error occurs. An `object`, being a singleton, " +
                "that extends any type of Throwable consequently introduces a global singleton exception whose " +
                "instance may be inadvertently reused from multiple places, thus introducing shared mutable state.",
        )
    }

    // Supertypes are read from their own lookup tags, so no class id is
    // resolved from a symbol that may be local.
    private fun extendsThrowable(declaration: FirRegularClass, session: FirSession): Boolean =
        declaration.superTypeRefs.any { ref ->
            val type = ref.coneType.fullyExpandedType(session).lowerBoundIfFlexible() as? ConeClassLikeType
                ?: return@any false
            if (type.lookupTag.classId in throwableClassIds) return@any true
            val symbol = type.lookupTag.toClassSymbol(session) ?: return@any false
            inheritsFromAny(symbol, throwableClassIds, session, lookupInterfaces = false)
        }
}
