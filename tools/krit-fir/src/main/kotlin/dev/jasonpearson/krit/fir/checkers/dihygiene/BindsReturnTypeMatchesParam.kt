package dev.jasonpearson.krit.fir.checkers.dihygiene

import com.intellij.lang.LighterASTNode
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
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassLikeSymbol
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.expressions.FirLiteralExpression
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.types.ConeErrorType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.fir.types.isSubtypeOf
import org.jetbrains.kotlin.lexer.KtTokens
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name
import org.jetbrains.kotlin.text
import org.jetbrains.kotlin.util.getChildren

/**
 * Port of the Go BindsReturnTypeMatchesParam rule: a named function annotated
 * `@Binds` with exactly one value parameter whose type is the function's
 * declared return type, reported on the function's first line (its modifier
 * list, where `@Binds` sits), like Go. Top-level, member, interface, object,
 * anonymous-object, and local functions all count, as every Go
 * `function_declaration` does, abstract or not. An extension receiver or a
 * context parameter is not a value parameter, as in Go. A function without an
 * explicit return type is skipped, as in Go. The message interpolates the
 * parameter type as written, without its type modifiers, which is Go's text.
 *
 * The parameter and return types match when they are the same type after
 * type alias expansion, nullability included; type annotations
 * (`@JvmSuppressWildcards`) do not count, as Go drops them from the type text.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`BindsReturnTypeMatchesParam*.kt`):
 * - Go matches `@Binds` as a substring of the function's modifier text, so it
 *   also reports a `@BindsInstance` or `@BindsOptionalOf` function and a
 *   function annotated with a user annotation class named `Binds`. Only
 *   Dagger's and Metro's `@Binds` declare a binding, so only those count.
 * - Go compares the type text alone. A binding whose function and parameter
 *   carry different qualifiers (`@Named("a")` on one side only) aliases one
 *   key to another, and a multibinding contribution (`@IntoSet`, `@IntoMap`,
 *   `@ElementsIntoSet`) binds into a collection, so neither is a no-op and
 *   neither is reported here. A `vararg` parameter's type is an array, and a
 *   `suspend` function type is not the plain function type Go reads once it
 *   drops the `suspend` modifier, so those types do not match.
 * - Resolution sees matching types Go's text comparison misses: a qualified
 *   name against a simple one, a type alias, different whitespace, and an
 *   import alias or fully qualified use of `@Binds`. Go also counts the named
 *   parameters of a function-typed parameter (`(x: Foo) -> Unit`) as
 *   parameters of the binding, so it skips that single-parameter binding.
 */
internal object BindsReturnTypeMatchesParam :
    FirDeclarationChecker<FirNamedFunction>(MppCheckerKind.Common), FirRule {
    override val ruleId = "BindsReturnTypeMatchesParam"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val simpleFunctionCheckers = setOf(BindsReturnTypeMatchesParam)
    }

    private val binds = setOf(
        ClassId(FqName("dagger"), Name.identifier("Binds")),
        ClassId(FqName("dev.zacsweers.metro"), Name.identifier("Binds")),
    )

    private val multibindings = setOf(
        ClassId(FqName("dagger.multibindings"), Name.identifier("IntoSet")),
        ClassId(FqName("dagger.multibindings"), Name.identifier("IntoMap")),
        ClassId(FqName("dagger.multibindings"), Name.identifier("ElementsIntoSet")),
        ClassId(FqName("dev.zacsweers.metro"), Name.identifier("IntoSet")),
        ClassId(FqName("dev.zacsweers.metro"), Name.identifier("IntoMap")),
        ClassId(FqName("dev.zacsweers.metro"), Name.identifier("ElementsIntoSet")),
    )

    private val qualifierMarkers = setOf(
        ClassId(FqName("javax.inject"), Name.identifier("Qualifier")),
        ClassId(FqName("jakarta.inject"), Name.identifier("Qualifier")),
        ClassId(FqName("dev.zacsweers.metro"), Name.identifier("Qualifier")),
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirNamedFunction) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        val session = context.session
        val annotationIds = declaration.annotations.mapNotNull { it.toAnnotationClassId(session) }
        if (annotationIds.none { it in binds }) return
        if (annotationIds.any { it in multibindings }) return
        val parameter = declaration.valueParameters.singleOrNull() ?: return
        if (parameter.isVararg) return
        val returnSource = declaration.returnTypeRef.source ?: return
        if (returnSource.kind !is KtRealSourceElementKind) return
        val parameterSource = parameter.returnTypeRef.source ?: return
        if (parameterSource.kind !is KtRealSourceElementKind) return

        val parameterType = parameter.returnTypeRef.coneType.fullyExpandedType()
        val returnType = declaration.returnTypeRef.coneType.fullyExpandedType()
        if (parameterType is ConeErrorType || returnType is ConeErrorType) return
        // Equal types: each is a subtype of the other.
        if (!parameterType.isSubtypeOf(returnType, session) || !returnType.isSubtypeOf(parameterType, session)) return
        if (qualifiers(declaration.annotations, session) != qualifiers(parameter.annotations, session)) return

        val name = functionNameText(source) ?: declaration.name.asString()
        report(
            firstLineAnchor(source),
            "@Binds function '$name' has matching parameter and return type " +
                "'${typeText(parameterSource)}'; the binding is a no-op.",
        )
    }

    // Go reports the declaration's first line, where its modifier list starts
    // (a KDoc belongs to the function in the light tree, but not in Go's).
    // Anchor on the list's first annotation or modifier, which stays on that
    // line even when the list spans several.
    private fun firstLineAnchor(source: KtSourceElement): KtSourceElement {
        val modifiers = source.getChild(KtNodeTypes.MODIFIER_LIST, depth = 1) ?: return source
        val first = lightChildren(modifiers, modifiers.lighterASTNode).firstOrNull { it.isCode() }
            ?: return modifiers
        return lightSourceOf(first, modifiers)
    }

    private fun LighterASTNode.isCode(): Boolean =
        tokenType !in KtTokens.WHITESPACES && tokenType !in KtTokens.COMMENTS

    // The qualifier annotations among [annotations] (those whose class is
    // meta-annotated `@Qualifier`), each keyed by its class and arguments, so
    // two sides carry the same qualifier only when both agree.
    private fun qualifiers(annotations: List<FirAnnotation>, session: FirSession): Set<String> =
        annotations.mapNotNullTo(mutableSetOf()) { annotation ->
            val classId = annotation.toAnnotationClassId(session) ?: return@mapNotNullTo null
            val symbol = annotation.toAnnotationClassLikeSymbol(session) ?: return@mapNotNullTo null
            val isQualifier = symbol.resolvedAnnotationsWithClassIds.any {
                it.toAnnotationClassId(session) in qualifierMarkers
            }
            if (!isQualifier) return@mapNotNullTo null
            val arguments = annotation.argumentMapping.mapping.entries
                .sortedBy { it.key.asString() }
                .joinToString(",") { (argName, value) ->
                    val rendered = (value as? FirLiteralExpression)?.value?.toString()
                        ?: value.source?.text?.toString()
                    "$argName=$rendered"
                }
            "$classId($arguments)"
        }

    // The type as written without its type modifiers (annotations,
    // `suspend`), which is the text Go reads.
    private fun typeText(source: KtSourceElement): String {
        val node = source.lighterASTNode
        val children = lightChildren(source, node)
        val tree = source.treeStructure
        val first = children.firstOrNull { it.tokenType != KtNodeTypes.MODIFIER_LIST && it.isCode() }
        val text = source.text?.toString() ?: return tree.toString(node).toString().trim()
        if (first == null || first == children.firstOrNull()) return text.trim()
        return text.substring(first.startOffset - node.startOffset).trim()
    }

    // The name as written, backticks included, which is what Go interpolates.
    private fun functionNameText(source: KtSourceElement): String? {
        val tree = source.treeStructure
        val identifier = source.lighterASTNode.getChildren(tree)
            .firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return null
        return tree.toString(identifier).toString()
    }
}
