package dev.jasonpearson.krit.fir.checkers.dihygiene

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.lightChildren
import dev.jasonpearson.krit.fir.support.lightSourceOf
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.analysis.getChild
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.toClassLikeSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go IntoMapMissingKey rule: a named function annotated with
 * Dagger's (or Metro's) `@IntoMap` and a binding annotation that has no map
 * key annotation, reported on the function's first line (its first
 * annotation), like Go, with the written function name. Top-level, member,
 * abstract, interface, object, companion, anonymous-object, and local
 * functions all count, as every Go `function_declaration` does.
 *
 * Go's binding test is the text `@Provides` or `@Binds` at the start of an
 * annotation, so a binding annotation here is any annotation whose written
 * name (an import alias's own name included), type alias name, or class name
 * starts with `Provides` or `Binds`: Dagger's and Metro's `@Provides` and
 * `@Binds`, Dagger's `@BindsInstance` and `@BindsOptionalOf`, and project
 * annotations named or import-aliased that way. A map key annotation is one
 * whose written, type alias, or class name ends in `Key` (other than `Key`
 * and `MapKey`), the name test Go applies, or one whose class is
 * meta-annotated with Dagger's or Metro's `@MapKey`, the annotation Dagger
 * itself requires.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`IntoMapMissingKey*.kt`):
 * - Go matches `@IntoMap`, `@Provides`, and `@Binds` as substrings of the
 *   annotation text, so it reports a function whose `@IntoMap` is a project
 *   or other-framework lookalike (kotlin-inject's `@IntoMap` contributes a
 *   `Pair` and needs no key), whose annotation name only starts with
 *   `IntoMap`, or whose `@IntoMap` text sits inside another annotation's
 *   string argument. None of those is a Dagger map contribution, so none is
 *   reported here.
 * - Go reads the written key name, so it reports a function whose key is an
 *   import or type alias (`@SK("x")` for `StringKey`), a `@MapKey` annotation
 *   whose name does not end in `Key` or is exactly `Key`, or a key inside a
 *   bracketed group that starts with another annotation
 *   (`@[Named("x") StringKey("a")]`, whose name Go reads as `[Named`). Those
 *   functions have a map key, so they are not reported here.
 * - Go also finds `@Provides` or `@Binds` inside another annotation's string
 *   argument (`@Named("@Provides")`), so it reports a Dagger `@IntoMap`
 *   function that has no binding annotation. That function is not a map
 *   contribution, so it is not reported here.
 * - Go misses a fully qualified, import-aliased, or type-aliased `@IntoMap`,
 *   `@Provides`, or `@Binds`, and the bracketed `@[Provides IntoMap]` and
 *   `@[IntoMap]` forms.
 *   Those are map contributions without a key, so they are reported here.
 */
internal object IntoMapMissingKey : FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "IntoMapMissingKey"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(IntoMapMissingKey)
    }

    private val dagger = FqName("dagger")
    private val daggerMultibindings = FqName("dagger.multibindings")
    private val metro = FqName("dev.zacsweers.metro")

    private val intoMap = setOf(
        ClassId(daggerMultibindings, Name.identifier("IntoMap")),
        ClassId(metro, Name.identifier("IntoMap")),
    )

    private val mapKey = setOf(
        ClassId(dagger, Name.identifier("MapKey")),
        ClassId(metro, Name.identifier("MapKey")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val session = context.session
        val annotations = declaration.annotations
        if (annotations.none { expandedType(it, session)?.lookupTag?.classId in intoMap }) return
        if (annotations.none { isBindingAnnotation(it, session) }) return
        if (annotations.any { isMapKey(it, session) }) return
        val name = functionNameText(source) ?: declaration.name.asString()
        report(
            firstLineAnchor(source),
            "@IntoMap function '$name' is missing a @*Key annotation; " +
                "Dagger requires a key annotation on every map contribution.",
        )
    }

    // Go's binding test: an annotation whose name starts with Provides or
    // Binds. The @IntoMap class id already ties the function to Dagger or
    // Metro.
    private fun isBindingAnnotation(annotation: FirAnnotation, session: FirSession): Boolean =
        annotationNames(annotation, session).any { it.startsWith("Provides") || it.startsWith("Binds") }

    // The names Go's name tests see and the resolved names: the short name
    // written in the source (an import alias's own name, which the resolved
    // type does not keep), the type alias name when it is written through one,
    // and the annotation's class name.
    private fun annotationNames(annotation: FirAnnotation, session: FirSession): List<String> {
        val names = mutableListOf<String>()
        writtenShortName(annotation)?.let(names::add)
        val written = annotation.annotationTypeRef.coneType as? ConeClassLikeType ?: return names
        val expanded = expandedType(annotation, session) ?: written
        names += written.lookupTag.name.asString()
        names += expanded.lookupTag.name.asString()
        return names
    }

    // The last identifier of the annotation's user type as written, `BarKey`
    // for `@BarKey` under `import p.Bar as BarKey` and `StringKey` for
    // `@dagger.multibindings.StringKey`: the name Go reads.
    private fun writtenShortName(annotation: FirAnnotation): String? {
        val source = annotation.source ?: return null
        if (source.kind !is KtRealSourceElementKind) return null
        var node = source.lighterASTNode
        if (node.tokenType != KtNodeTypes.ANNOTATION_ENTRY) return null
        for (type in writtenTypePath) {
            node = lightChildren(source, node).firstOrNull { it.tokenType == type } ?: return null
        }
        val reference = lightChildren(source, node).lastOrNull { it.tokenType == KtNodeTypes.REFERENCE_EXPRESSION }
            ?: return null
        val identifier = lightChildren(source, reference).firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
            ?: return null
        return source.treeStructure.toString(identifier).toString().removeSurrounding("`")
    }

    private val writtenTypePath = listOf(KtNodeTypes.CONSTRUCTOR_CALLEE, KtNodeTypes.TYPE_REFERENCE, KtNodeTypes.USER_TYPE)

    // The annotation's class type, through any type alias.
    private fun expandedType(annotation: FirAnnotation, session: FirSession): ConeClassLikeType? =
        annotation.annotationTypeRef.coneType.fullyExpandedType(session) as? ConeClassLikeType

    private fun isMapKey(annotation: FirAnnotation, session: FirSession): Boolean {
        // Go's name test.
        if (annotationNames(annotation, session).any(::isKeyName)) return true
        val type = expandedType(annotation, session) ?: return false
        // The annotation class's own annotations, read from its lookup tag so
        // no class id is resolved.
        val symbol = type.lookupTag.toClassLikeSymbol(session) ?: return false
        return symbol.resolvedAnnotationClassIds.any { it in mapKey }
    }

    private fun isKeyName(name: String): Boolean = name != "Key" && name != "MapKey" && name.endsWith("Key")

    // Go reports the declaration's first line: its first annotation, where
    // the modifier list starts (a preceding KDoc is not part of Go's node).
    private fun firstLineAnchor(source: KtSourceElement): KtSourceElement {
        val modifiers = source.getChild(KtNodeTypes.MODIFIER_LIST, depth = 1) ?: return source
        val first = lightChildren(modifiers, modifiers.lighterASTNode).firstOrNull {
            it.tokenType != KtTokens.WHITE_SPACE && it.tokenType !in KtTokens.COMMENTS
        } ?: return modifiers
        return lightSourceOf(first, modifiers)
    }

    // The function name as written (backticks included), as Go reads it.
    private fun functionNameText(source: KtSourceElement): String? {
        val identifier = lightChildren(source, source.lighterASTNode)
            .firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        return source.treeStructure.toString(identifier).toString()
    }
}
