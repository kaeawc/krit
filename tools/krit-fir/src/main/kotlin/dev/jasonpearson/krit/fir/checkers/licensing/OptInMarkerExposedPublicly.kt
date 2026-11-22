package dev.jasonpearson.krit.fir.checkers.licensing

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.isInTestFile
import dev.jasonpearson.krit.fir.report
import dev.jasonpearson.krit.fir.support.classIdOf
import dev.jasonpearson.krit.fir.support.declarationStatus
import dev.jasonpearson.krit.fir.support.hasDirectSupertypeAnnotatedWith
import dev.jasonpearson.krit.fir.support.isEffectivelyPublicApi
import dev.jasonpearson.krit.fir.support.kDocText
import dev.jasonpearson.krit.fir.support.resolvedAnnotationClass
import dev.jasonpearson.krit.fir.support.writtenAnnotationName
import org.jetbrains.kotlin.KtRealSourceElementKind
import org.jetbrains.kotlin.diagnostics.DiagnosticReporter
import org.jetbrains.kotlin.fir.analysis.checkers.MppCheckerKind
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.DeclarationCheckers
import org.jetbrains.kotlin.fir.analysis.checkers.declaration.FirDeclarationChecker
import org.jetbrains.kotlin.fir.declarations.FirConstructor
import org.jetbrains.kotlin.fir.declarations.FirDeclaration
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.FirProperty
import org.jetbrains.kotlin.fir.declarations.FirRegularClass
import org.jetbrains.kotlin.fir.declarations.FirTypeAlias
import org.jetbrains.kotlin.fir.symbols.impl.FirFileSymbol
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.symbols.FirBasedSymbol

/**
 * Confirms or removes Go findings; the shared verdict never adds FIR-only candidates. Resolved
 * marker simple names match the shared well-known table or configured additionalMarkers, just
 * like Go. RequiresOptIn metadata must resolve to kotlin.RequiresOptIn, rejecting lookalikes.
 * RequiresOptIn itself is plumbing, not a default marker; explicitly configured names still apply.
 *
 * Only well-known library markers are skipped within their package family. Known families
 * distinguish Coil and Kotlin siblings and Accompanist/Horologist from unrelated com.google apps;
 * other well-known markers use their first two package segments. Explicitly configured markers
 * bypass this heuristic because projects opt into checking them even when their APIs share a
 * package root.
 */
internal object OptInMarkerExposedPublicly : FirDeclarationChecker<FirDeclaration>(MppCheckerKind.Common), FirRule {
    override val ruleId = "OptInMarkerExposedPublicly"
    override val declarationCheckers = object : DeclarationCheckers() {
        override val basicDeclarationCheckers = setOf(OptInMarkerExposedPublicly)
    }

    private val requiresOptIn = classIdOf("kotlin", "RequiresOptIn")
    private val optIn = classIdOf("kotlin", "OptIn")
    private val acknowledgmentWords = listOf(
        "opt in", "opt-in", "optin", "experimental", "unstable", "warning", "may change",
    )

    context(context: CheckerContext, reporter: DiagnosticReporter)
    override fun check(declaration: FirDeclaration) {
        val source = declaration.source ?: return
        if (source.kind !is KtRealSourceElementKind || isInTestFile() || !eligible(declaration)) return
        val symbol = declaration.symbol
        val owners = publicOwners(symbol)
        val declarationPackageName = declaration.packageName()
        if (owners == null || declarationPackageName == null) return
        val doc = kDocText(source)?.lowercase()
        for (annotation in declaration.annotations) {
            val name = exposedMarkerName(annotation, declarationPackageName, doc, owners + symbol) ?: continue
            report(
                annotation.source,
                "@$name on a public declaration propagates the opt-in requirement to callers. " +
                    "Restrict the declaration's visibility or wrap the experimental usage " +
                    "behind a non-experimental API.",
            )
        }
    }

    private fun eligible(declaration: FirDeclaration): Boolean {
        val supported = when (declaration) {
            is FirRegularClass, is FirNamedFunction, is FirProperty, is FirTypeAlias -> true
            is FirConstructor -> !declaration.isPrimary
            else -> false
        }
        val symbol = declaration.symbol
        return supported && declaration.annotations.isNotEmpty() &&
            symbol.isEffectivelyPublicApi() && symbol.declarationStatus()?.isOverride != true
    }

    context(context: CheckerContext)
    private fun publicOwners(symbol: FirBasedSymbol<*>): List<FirBasedSymbol<*>>? {
        val owners = context.containingDeclarations.filter { it !== symbol && it !is FirFileSymbol }
        val hiddenOwner = owners.any { it !is FirRegularClassSymbol || !it.isEffectivelyPublicApi() }
        return owners.takeUnless { hiddenOwner }
            ?.takeUnless { (it + symbol).any { owner -> optIn in owner.resolvedAnnotationClassIds } }
    }

    context(context: CheckerContext)
    private fun exposedMarkerName(
        annotation: FirAnnotation,
        packageName: String,
        doc: String?,
        owners: List<FirBasedSymbol<*>>,
    ): String? {
        val marker = annotation.resolvedAnnotationClass(context.session) ?: return null
        val markerClassId = marker.classId
        val simpleName = markerClassId.shortClassName.asString()
        val name = annotation.writtenAnnotationName() ?: simpleName
        val documented = doc != null && (name.lowercase() in doc || acknowledgmentWords.any { it in doc })
        val inherited = owners.filterIsInstance<FirRegularClassSymbol>().any {
                it.hasDirectSupertypeAnnotatedWith(markerClassId, context.session)
            }
        val additional = (config()["additionalMarkers"] as? List<*>)
            .orEmpty().filterIsInstance<String>().map { it.substringAfterLast('.') }
        val configured = simpleName in additional
        val wellKnown = simpleName != "RequiresOptIn" && simpleName in wellKnownOptInMarkers
        val tracked = requiresOptIn in marker.resolvedAnnotationClassIds && (wellKnown || configured)
        if (!tracked || documented || inherited) return null
        if (!configured && wellKnown &&
            isInOptInMarkerPackageFamily(packageName, markerClassId.packageFqName.asString())
        ) return null
        return name
    }

    private fun FirDeclaration.packageName(): String? = when (this) {
        is FirRegularClass -> symbol.classId.packageFqName.asString()
        is FirTypeAlias -> symbol.classId.packageFqName.asString()
        is FirNamedFunction -> symbol.callableId?.packageName?.asString()
        is FirProperty -> symbol.callableId?.packageName?.asString()
        is FirConstructor -> symbol.callableId?.packageName?.asString()
        else -> null
    }

}
