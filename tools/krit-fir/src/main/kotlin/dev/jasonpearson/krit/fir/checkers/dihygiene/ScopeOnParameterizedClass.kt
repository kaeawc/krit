package dev.jasonpearson.krit.fir.checkers.dihygiene

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.firstModifierAnchor
import dev.jasonpearson.krit.fir.support.identifierText
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.FirSession
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirRegularClassChecker
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.FirTypeParameter
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassIdSafe
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.resolve.fullyExpandedType
import org.jetbrains.kotlin.fir.resolve.toClassSymbol
import org.jetbrains.kotlin.fir.types.ConeClassLikeType
import org.jetbrains.kotlin.fir.types.coneType

/**
 * Port of the Go ScopeOnParameterizedClass rule: a class or interface that
 * declares its own type parameters and carries a DI scope annotation, reported
 * on the declaration's first line (its modifier list), like Go. The message
 * names the scope annotation and the class.
 *
 * Koin's `@Singleton`/`@Single`, EJB's `@Singleton` and JSF managed-bean scopes share an
 * instance across type arguments. CDI normal scopes, identified through
 * `NormalScope`, instead make a generic bean a definition error: only
 * `@Dependent` is allowed. Dagger 2.60.1, Metro 1.5.1 and Guice 5.1.0 key
 * scoped bindings by full type, so their scopes are not reported.
 * With several supported scopes, the first in source order names the finding.
 * The class name is the identifier as written, backticks included, as in Go.
 *
 * Deliberate differences from Go, each pinned in the golden data
 * (`ScopeOnParameterizedClass*.kt`):
 * - Go requires a qualified reference, an explicit import (including aliases),
 *   or a scope declaration in the file's own framework package. It does not
 *   resolve wildcard imports, type aliases or custom CDI normal scopes.
 */
internal object ScopeOnParameterizedClass :
    FirRegularClassChecker(MppCheckerKind.Common), FirRule {
    override val ruleId = "ScopeOnParameterizedClass"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val regularClassCheckers = setOf(ScopeOnParameterizedClass)
    }

    private enum class ScopeKind { SHARED, CDI }

    private val scopeMetaAnnotations = setOf(
        classIdOf("javax.enterprise.context", "NormalScope"),
        classIdOf("jakarta.enterprise.context", "NormalScope"),
    )

    // Scope annotations that carry no scope meta-annotation: Koin's single
    // definition, EJB's singleton session bean (one instance per application)
    // and the JSF managed-bean scopes, which Go reports from resolved framework packages.
    private val scopeAnnotations = setOf(
        classIdOf("org.koin.core.annotation", "Singleton"),
        classIdOf("org.koin.core.annotation", "Single"),
        classIdOf("javax.ejb", "Singleton"),
        classIdOf("jakarta.ejb", "Singleton"),
        classIdOf("javax.faces.bean", "ApplicationScoped"),
        classIdOf("javax.faces.bean", "SessionScoped"),
        classIdOf("javax.faces.bean", "RequestScoped"),
        classIdOf("javax.faces.bean", "ViewScoped"),
        classIdOf("jakarta.faces.bean", "ApplicationScoped"),
        classIdOf("jakarta.faces.bean", "SessionScoped"),
        classIdOf("jakarta.faces.bean", "RequestScoped"),
        classIdOf("jakarta.faces.bean", "ViewScoped"),
    )

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
        val (scope, kind) = scopes.first()
        // The class name as written, backticks included, like Go's identifier text.
        val name = identifierText(source) ?: declaration.name.asString()
        // Go reports the declaration's first line, where its modifier list
        // (the scope annotation included) starts.
        report(
            firstModifierAnchor(source) ?: source,
            when (kind) {
                ScopeKind.SHARED -> "@$scope on generic class '$name' shares one instance across all type " +
                    "arguments because the type parameter is erased at runtime."
                ScopeKind.CDI -> "@$scope on generic class '$name': CDI allows only @Dependent on a bean " +
                    "with type parameters, so this is a definition error."
            },
        )
    }

    // The simple name and kind of the annotation's class when it is a supported scope, else
    // null. The annotation type is read from its own lookup tag (annotation
    // classes are never local), expanding a type alias.
    private fun scopeName(annotation: FirAnnotation, session: FirSession): Pair<String, ScopeKind>? {
        val type = annotation.annotationTypeRef.coneType.fullyExpandedType(session) as? ConeClassLikeType ?: return null
        val classId = type.lookupTag.classId
        val name = classId.shortClassName.asString()
        if (classId in scopeAnnotations) return name to ScopeKind.SHARED
        val symbol = type.lookupTag.toClassSymbol(session) ?: return null
        val isScope = symbol.resolvedAnnotationsWithClassIds.any {
            it.toAnnotationClassIdSafe(session) in scopeMetaAnnotations
        }
        return if (isScope) name to ScopeKind.CDI else null
    }
}
