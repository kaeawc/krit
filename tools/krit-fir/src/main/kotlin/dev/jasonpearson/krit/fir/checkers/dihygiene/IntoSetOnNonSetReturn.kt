package dev.jasonpearson.krit.fir.checkers.dihygiene

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.identifierText
import dev.jasonpearson.krit.fir.support.lightChildren
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.lookupSuperTypes
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.ConeKotlinType
import org.jetbrains.kotlin.fir.types.ConeTypeParameterType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text

/**
 * Port of the Go IntoSetOnNonSetReturn rule: a named function annotated
 * `@IntoSet` and `@Provides` or `@Binds` whose declared return type is a
 * collection wrapper (`List`, `MutableList`, `ArrayList`, `Set`,
 * `MutableSet`, `HashSet`, `LinkedHashSet`, `Map`, `MutableMap`, `HashMap`,
 * `LinkedHashMap`, `Collection`, `MutableCollection`, `Iterable`,
 * `MutableIterable`, or `Array`), nullable or not. It is reported on the
 * function's first line (its modifier list), like Go. Every named function
 * counts, as every Go `function_declaration` does: top-level, member,
 * interface, object, companion, local, extension, and abstract functions
 * (a `@Binds` has no body). Property accessors and functions without a
 * declared return type are skipped, as in Go.
 *
 * The annotations are matched by class id: Dagger's `dagger.multibindings
 * .IntoSet` with `dagger.Provides` / `dagger.Binds`, and the same-named
 * annotations of Metro (`dev.zacsweers.metro`) and kotlin-inject
 * (`me.tatarka.inject.annotations`), which also collect a set by the
 * provider's return type. Go's wrapper names still select the return type, as
 * in Go, from whichever package the class comes from (a project's own `Set`,
 * Eclipse Collections' `MutableList`, Vavr's `HashMap`), but the type must
 * really be a collection: a class that is, or extends, an `Iterable` or a
 * `Map`, `kotlin.Array`, or a type parameter bounded by one.
 *
 * Deliberate differences from Go, which matches the annotation text
 * (`@IntoSet` anywhere in the modifier list) and the last dotted segment of
 * the declared type's text (see the golden data, `IntoSetOnNonSetReturn*.kt`):
 * - Go reports same-named annotations that are not a DI framework's (a local
 *   `annotation class IntoSet`), annotation text that is not an annotation
 *   (`@Named("@IntoSet")`) or is a different one (`@BindsOptionalOf`
 *   contains `@Binds`), a declared type whose name matches but is not a
 *   collection (a local `class List`, a type alias named `Collection`, a
 *   nested `Holder.Set`, a type parameter named `Set` bounded by a
 *   non-collection), and a function type whose text contains a collection
 *   name (`() -> kotlin.collections.List<T>`). None contributes a collection
 *   wrapper to a multibound set, so none is reported here.
 * - Resolution sees a collection wrapper Go misses: a type alias or import
 *   alias of a collection, a parenthesized collection type, and an
 *   annotation written by its fully qualified name, through an import alias,
 *   or in the `@[Provides IntoSet]` form.
 */
internal object IntoSetOnNonSetReturn :
    FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "IntoSetOnNonSetReturn"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(IntoSetOnNonSetReturn)
    }

    private val dagger = FqName("dagger")
    private val daggerMultibindings = FqName("dagger.multibindings")
    private val metro = FqName("dev.zacsweers.metro")
    private val kotlinInject = FqName("me.tatarka.inject.annotations")

    private val intoSet = setOf(
        classId(daggerMultibindings, "IntoSet"),
        classId(metro, "IntoSet"),
        classId(kotlinInject, "IntoSet"),
    )
    private val providesOrBinds = setOf(
        classId(dagger, "Provides"),
        classId(dagger, "Binds"),
        classId(metro, "Provides"),
        classId(metro, "Binds"),
        classId(kotlinInject, "Provides"),
    )

    // Go's wrapper names. Go matches the last dotted segment of the declared
    // type's text against them, so any class with one of these names counts,
    // wherever it is declared (a project's own `Set`, Eclipse Collections'
    // `MutableList`, Vavr's `HashMap`). The checker keeps that, but only for a
    // type that really is a collection (see [isCollection]).
    private val wrapperNames = setOf(
        "List", "MutableList", "ArrayList", "Set", "MutableSet", "HashSet", "LinkedHashSet",
        "Map", "MutableMap", "HashMap", "LinkedHashMap", "Collection", "MutableCollection",
        "Iterable", "MutableIterable", "Array",
    )

    private val kotlinArray = classId(FqName("kotlin"), "Array")

    // A type is a collection when it is, or has as a supertype, one of these.
    // Kotlin maps the Java interfaces onto its own, but a Kotlin file can
    // still name the Java ones explicitly.
    private val collectionRoots = setOf(
        classId(FqName("kotlin.collections"), "Iterable"),
        classId(FqName("kotlin.collections"), "MutableIterable"),
        classId(FqName("kotlin.collections"), "Map"),
        classId(FqName("kotlin.collections"), "MutableMap"),
        classId(FqName("java.lang"), "Iterable"),
        classId(FqName("java.util"), "Collection"),
        classId(FqName("java.util"), "Map"),
    )

    private fun classId(pkg: FqName, name: String) = ClassId(pkg, Name.identifier(name))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val annotations = declaration.annotations.mapNotNull { it.toAnnotationClassId(context.session) }
        if (annotations.none { it in intoSet }) return
        if (annotations.none { it in providesOrBinds }) return

        // Go reads only a declared return type.
        val typeRef = declaration.returnTypeRef
        val typeSource = typeRef.source ?: return
        if (typeSource.kind !is KtRealSourceElementKind) return
        val typeText = writtenTypeText(typeSource)
        val wrapper = collectionWrapper(typeRef.coneType, typeText, context.session) ?: return

        val name = identifierText(source) ?: declaration.name.asString()
        report(
            source,
            "@IntoSet function '$name' returns '$typeText', a collection wrapper; the DI framework collects it by return type, " +
                "so the contribution will be a Set<$wrapper> entry rather than the intended elements.",
        )
    }

    // The wrapper name the message quotes, or null when the declared type is
    // not a collection wrapper. The name is the first of these in Go's list:
    // the last dotted segment of the written type (Go's name, which also
    // covers an import alias such as `LinkedList as List`), the name of the
    // class, type alias, or type parameter it resolves to, and the name of the
    // class behind a type alias (`typealias Plugins = List<Plugin>`). The
    // type must be a collection: a class that is, or extends, an Iterable or
    // a Map, `kotlin.Array`, or a type parameter bounded by one.
    // A function type (`() -> List<T>`) resolves to `kotlin.FunctionN`, which
    // is not a collection.
    private fun collectionWrapper(type: ConeKotlinType, typeText: String, session: FirSession): String? {
        val declared = type.lowerBoundIfFlexible()
        val declaredName = when (declared) {
            is ConeClassLikeType -> declared.lookupTag.name.asString()
            is ConeTypeParameterType -> declared.lookupTag.name.asString()
            else -> return null
        }
        val expanded = declared.fullyExpandedType(session).lowerBoundIfFlexible()
        val expandedName = (expanded as? ConeClassLikeType)?.lookupTag?.name?.asString()
        val wrapper = listOfNotNull(goWrapperName(typeText), declaredName, expandedName)
            .firstOrNull { it in wrapperNames } ?: return null
        return wrapper.takeIf { isCollection(expanded, session, depth = 0) }
    }

    // Go's reading of the type text: drop a trailing `?`, cut at the first
    // `<`, and keep the last dotted segment.
    private fun goWrapperName(typeText: String): String {
        val bare = typeText.trim().removeSuffix("?").trim().substringBefore('<').trim()
        return bare.substringAfterLast('.')
    }

    // Supertypes are read through the type's own lookup tag, which is bound to
    // local classes, so no class id is resolved through the symbol provider.
    private fun isCollection(type: ConeKotlinType, session: FirSession, depth: Int): Boolean {
        if (depth > MAX_BOUND_DEPTH) return false
        return when (val t = type.fullyExpandedType(session).lowerBoundIfFlexible()) {
            is ConeClassLikeType -> {
                val classId = t.lookupTag.classId
                if (classId == kotlinArray || classId in collectionRoots) return true
                val symbol = t.lookupTag.toClassSymbol(session) ?: return false
                lookupSuperTypes(symbol, lookupInterfaces = true, deep = true, useSiteSession = session)
                    .any { it.lookupTag.classId in collectionRoots }
            }
            is ConeTypeParameterType -> t.lookupTag.typeParameterSymbol.resolvedBounds.any {
                isCollection(it.coneType, session, depth + 1)
            }
            else -> false
        }
    }

    private const val MAX_BOUND_DEPTH = 8

    // The declared type as Go quotes it: the type reference's text without
    // any type annotations in front of it (`@JvmSuppressWildcards List<T>`
    // is quoted as `List<T>`).
    private fun writtenTypeText(typeSource: KtSourceElement): String {
        val text = typeSource.text?.toString() ?: return ""
        val node = typeSource.lighterASTNode
        val modifiers = lightChildren(typeSource, node).firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST }
            ?: return text.trim()
        val cut = (modifiers.endOffset - node.startOffset).coerceIn(0, text.length)
        return text.substring(cut).trim()
    }
}
