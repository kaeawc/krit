package dev.jasonpearson.krit.fir.checkers.dihygiene

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.identifierText
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.analysis.getChild
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.FirTypeParameter
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassIdSafe
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType
import org.jetbrains.kotlin.name.ClassId
import org.jetbrains.kotlin.name.FqName
import org.jetbrains.kotlin.name.Name

/**
 * Port of the Go ScopeOnParameterizedClass rule: a class or interface that
 * declares its own type parameters and carries a DI scope annotation, reported
 * on the declaration's first line (its modifier list), like Go. The message
 * names the scope annotation and the class.
 *
 * An annotation is a DI scope when its class is meta-annotated as one
 * (`javax.inject.Scope`, `jakarta.inject.Scope`, Guice's `ScopeAnnotation`,
 * CDI's `NormalScope`, kotlin-inject's and Metro's `Scope`), which covers
 * `javax`/`jakarta` `@Singleton`, Dagger's `@Reusable`, the Hilt scopes, Guice
 * and CDI scopes, and project scopes, or when it is Koin's `@Singleton`
 * definition annotation, EJB's `@Singleton` or a JSF managed-bean scope. The
 * pseudo-scopes CDI `@Dependent` and Micronaut `@Prototype` are meta-annotated
 * as scopes but create an instance per injection point, so they do not count.
 * Go takes the first name of its scope list that appears as `@<Name>` in the
 * class's modifier text; with several scopes the message names the one that
 * comes first in that list, as in Go. The class name is the identifier as
 * written, backticks included, as in Go.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`ScopeOnParameterizedClass*.kt`):
 * - Go matches the text `@<Name>` anywhere in the modifier list, as a prefix,
 *   so it reports a generic class whose annotation merely starts with a scope
 *   name (`@SingletonHolder`), mentions one in a string argument
 *   (`@Named("@Singleton")`) or in a comment between annotations, or is a
 *   same-named annotation that is not a DI scope (a local
 *   `annotation class Singleton` without `@Scope`). None of those classes is
 *   scoped, so none is reported here. When such a lookalike sits next to a
 *   real scope (`@SingletonHolder @ActivityScoped`), both report the line, but
 *   Go's message names the lookalike's list entry (`@Singleton`) and this one
 *   names the scope that is there (`@ActivityScoped`).
 * - Go only knows the simple names in its list written without a qualifier,
 *   so it misses `@javax.inject.Singleton`, an import alias, a type alias, a
 *   project scope with another name (`@PerActivity`), and a `fun interface`
 *   (whose declaration tree-sitter does not parse as a class). Those are
 *   scoped generic types and are reported here.
 */
internal object ScopeOnParameterizedClass :
    FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ScopeOnParameterizedClass"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(ScopeOnParameterizedClass)
    }

    // Go's scope-name list, in Go's order: it decides which name the message
    // uses when a class carries more than one scope.
    private val goScopeNames = listOf(
        "Singleton",
        "Reusable",
        "ApplicationScoped",
        "ActivityScoped",
        "ActivityRetainedScoped",
        "FragmentScoped",
        "ViewScoped",
        "ViewModelScoped",
        "ServiceScoped",
        "SessionScoped",
        "RequestScoped",
        "UserScoped",
    )

    private val scopeMetaAnnotations = setOf(
        classId("javax.inject", "Scope"),
        classId("jakarta.inject", "Scope"),
        classId("com.google.inject", "ScopeAnnotation"),
        classId("javax.enterprise.context", "NormalScope"),
        classId("jakarta.enterprise.context", "NormalScope"),
        classId("me.tatarka.inject.annotations", "Scope"),
        classId("dev.zacsweers.metro", "Scope"),
    )

    // Scope annotations that carry no scope meta-annotation: Koin's single
    // definition, EJB's singleton session bean (one instance per application)
    // and the JSF managed-bean scopes, which Go reports by their simple names.
    private val scopeAnnotations = setOf(
        classId("org.koin.core.annotation", "Singleton"),
        classId("javax.ejb", "Singleton"),
        classId("jakarta.ejb", "Singleton"),
        classId("javax.faces.bean", "ApplicationScoped"),
        classId("javax.faces.bean", "SessionScoped"),
        classId("javax.faces.bean", "RequestScoped"),
        classId("javax.faces.bean", "ViewScoped"),
        classId("jakarta.faces.bean", "ApplicationScoped"),
        classId("jakarta.faces.bean", "SessionScoped"),
        classId("jakarta.faces.bean", "RequestScoped"),
        classId("jakarta.faces.bean", "ViewScoped"),
    )

    // Pseudo-scopes meta-annotated as scopes that create a new instance for
    // every injection point, so no instance is shared across type arguments:
    // CDI's `@Dependent` and Micronaut's `@Prototype`.
    private val excludedScopes = setOf(
        classId("javax.enterprise.context", "Dependent"),
        classId("jakarta.enterprise.context", "Dependent"),
        classId("io.micronaut.context.annotation", "Prototype"),
    )

    private fun classId(pkg: String, name: String) = ClassId(FqName(pkg), Name.identifier(name))

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirRegularClass) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind) return
        // Only the class's own type parameters: an inner class also lists its
        // outer class's, which Go (reading the class's `type_parameters`) does
        // not count.
        if (declaration.typeParameters.none { it is FirTypeParameter }) return
        val scopes = declaration.annotations.mapNotNull { scopeName(it, context.session) }
        if (scopes.isEmpty()) return
        val scope = scopes.minBy { goScopeNames.indexOf(it).let { index -> if (index < 0) Int.MAX_VALUE else index } }
        // The class name as written, backticks included, like Go's identifier text.
        val name = identifierText(source) ?: declaration.name.asString()
        // Go reports the declaration's first line, where its modifier list
        // (the scope annotation included) starts.
        report(
            source.getChild(KtNodeTypes.MODIFIER_LIST, depth = 1) ?: source,
            "@$scope on generic class '$name' shares one instance across all type " +
                "arguments because the type parameter is erased at runtime.",
        )
    }

    // The simple name of the annotation's class when it is a DI scope, else
    // null. The annotation type is read from its own lookup tag (annotation
    // classes are never local), expanding a type alias.
    private fun scopeName(annotation: FirAnnotation, session: FirSession): String? {
        val type = annotation.annotationTypeRef.coneType.fullyExpandedType(session) as? ConeClassLikeType ?: return null
        val classId = type.lookupTag.classId
        val name = classId.shortClassName.asString()
        if (classId in excludedScopes) return null
        if (classId in scopeAnnotations) return name
        val symbol = type.lookupTag.toClassSymbol(session) ?: return null
        val isScope = symbol.resolvedAnnotationsWithClassIds.any {
            it.toAnnotationClassIdSafe(session) in scopeMetaAnnotations
        }
        return if (isScope) name else null
    }
}
