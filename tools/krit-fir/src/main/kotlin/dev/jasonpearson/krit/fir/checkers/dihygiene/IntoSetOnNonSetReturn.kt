package dev.jasonpearson.krit.fir.checkers.dihygiene

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.lowerBoundIfFlexible
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text
import org.jetbrains.kotlin.util.getChildren

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
 * provider's return type. The return type is matched by its resolved class
 * after type-alias expansion: the Kotlin collection interfaces, `kotlin.Array`,
 * the `java.util` classes behind the stdlib `ArrayList` / `HashSet` /
 * `LinkedHashSet` / `HashMap` / `LinkedHashMap` aliases, and the `java.util` /
 * `java.lang` interfaces Kotlin maps to its own collections, when written out.
 *
 * Deliberate differences from Go, which matches the annotation text
 * (`@IntoSet` anywhere in the modifier list) and the last dotted segment of
 * the declared type's text (see the golden data, `IntoSetOnNonSetReturn*.kt`):
 * - Go reports same-named annotations that are not a DI framework's (a local
 *   `annotation class IntoSet`), annotation text that is not an annotation
 *   (`@Named("@IntoSet")`) or is a different one (`@BindsOptionalOf`
 *   contains `@Binds`), and a declared type whose name matches but is not a
 *   collection (a local `class List`, a type alias named `Collection`, a
 *   nested `Holder.Set`, a type parameter named `Set`). None contributes a
 *   collection wrapper to a multibound set, so none is reported here.
 * - Resolution sees a collection wrapper Go misses: a type alias or import
 *   alias of a collection, a parenthesized collection type, and an
 *   annotation written by its fully qualified name or through an import
 *   alias.
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

    private val kotlinCollections = FqName("kotlin.collections")
    private val javaUtil = FqName("java.util")

    // Go's wrapper names, by the classes they resolve to.
    private val collectionWrappers: Set<ClassId> = buildSet {
        listOf(
            "List", "MutableList", "Set", "MutableSet", "Map", "MutableMap",
            "Collection", "MutableCollection", "Iterable", "MutableIterable",
        ).forEach { add(classId(kotlinCollections, it)) }
        add(classId(FqName("kotlin"), "Array"))
        // The targets of kotlin.collections.ArrayList & co., and the Java
        // interfaces a Kotlin file can still name explicitly.
        listOf(
            "ArrayList", "HashSet", "LinkedHashSet", "HashMap", "LinkedHashMap",
            "List", "Set", "Map", "Collection",
        ).forEach { add(classId(javaUtil, it)) }
        add(classId(FqName("java.lang"), "Iterable"))
    }

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
        val type = typeRef.coneType.fullyExpandedType().lowerBoundIfFlexible() as? ConeClassLikeType ?: return
        val classId = type.lookupTag.classId
        if (classId !in collectionWrappers) return
        val wrapper = classId.shortClassName.asString()

        val name = functionNameText(source) ?: declaration.name.asString()
        val typeText = writtenTypeText(typeSource)
        report(
            source,
            "@IntoSet function '$name' returns '$typeText', a collection wrapper; Dagger collects by return type, " +
                "so the contribution will be a Set<$wrapper> entry rather than the intended elements.",
        )
    }

    // The declared type as Go quotes it: the type reference's text without
    // any type annotations in front of it (`@JvmSuppressWildcards List<T>`
    // is quoted as `List<T>`).
    private fun writtenTypeText(typeSource: KtSourceElement): String {
        val text = typeSource.text?.toString() ?: return ""
        val tree = typeSource.treeStructure
        val node = typeSource.lighterASTNode
        val modifiers = node.getChildren(tree).firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST }
            ?: return text.trim()
        val cut = (modifiers.endOffset - node.startOffset).coerceIn(0, text.length)
        return text.substring(cut).trim()
    }

    // The name as written, backticks included, which is what Go interpolates.
    private fun functionNameText(source: KtSourceElement): String? {
        val tree = source.treeStructure
        val identifier = source.lighterASTNode.getChildren(tree)
            .firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        return tree.toString(identifier).toString()
    }
}
