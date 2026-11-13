package dev.jasonpearson.krit.fir.checkers.compose

import dev.jasonpearson.krit.fir.FirRule
import dev.jasonpearson.krit.fir.support.simpleName
import org.jetbrains.kotlin.fir.analysis.checkers.context.CheckerContext
import org.jetbrains.kotlin.fir.declarations.FirNamedFunction
import org.jetbrains.kotlin.fir.declarations.toAnnotationClassId
import org.jetbrains.kotlin.fir.expressions.FirAnnotation
import org.jetbrains.kotlin.fir.resolve.providers.symbolProvider
import org.jetbrains.kotlin.fir.symbols.impl.FirRegularClassSymbol

// Preview detection the Compose accessibility rules share. Go reads annotations and names as text on the nearest
// enclosing function; this reads the resolved annotation classes, so a multipreview annotation is found wherever
// it is declared.

/** The `customPreviewWildcard` and `customPreviewPrefixes` options of a Compose accessibility rule. */
internal class PreviewConfig(val wildcard: Boolean, val prefixes: List<String>)

internal fun FirRule.previewConfig(): PreviewConfig {
    val options = config()
    val wildcard = options["customPreviewWildcard"] as? Boolean ?: true
    val prefixes = (options["customPreviewPrefixes"] as? List<*>).orEmpty().filterIsInstance<String>()
    return PreviewConfig(wildcard, prefixes)
}

private val scaffoldingNamePrefixes = listOf("preview", "sample", "fake", "mock", "stub", "fixture")
private val scaffoldingNameSuffixes = listOf("preview", "sample", "fixture")

private fun isPreviewAnnotationName(simple: String, config: PreviewConfig): Boolean {
    if (simple.isEmpty()) return false
    if (simple == "Preview" || simple in androidxPreviewAnnotations) return true
    for (raw in config.prefixes) {
        val prefix = raw.trim()
        if (prefix.isNotEmpty() && (simple == prefix || simple == prefix + "Preview" || simple == prefix + "Previews")) return true
    }
    if (!config.wildcard) return false
    return simple.endsWith("Preview") || simple.endsWith("Previews") || simple.startsWith("Preview")
}

/**
 * True when this function is preview, sample, fake, mock, stub or fixture scaffolding: named like one, annotated with
 * a preview (honoring [config]), or annotated with a multipreview annotation whose class is itself a preview.
 */
context(context: CheckerContext)
internal fun FirNamedFunction.isPreviewScaffolding(config: PreviewConfig): Boolean {
    val lower = name.asString().lowercase()
    if (scaffoldingNamePrefixes.any { lower.startsWith(it) } || scaffoldingNameSuffixes.any { lower.endsWith(it) }) return true
    return annotations.any { isPreviewAnnotationName(it.simpleName().orEmpty(), config) || isMultipreview(it) }
}

context(context: CheckerContext)
private fun isMultipreview(annotation: FirAnnotation): Boolean {
    val classId = annotation.toAnnotationClassId(context.session) ?: return false
    val klass = context.session.symbolProvider.getClassLikeSymbolByClassId(classId) as? FirRegularClassSymbol ?: return false
    return klass.resolvedAnnotationsWithClassIds.any { inner ->
        val simple = inner.toAnnotationClassId(context.session)?.shortClassName?.asString().orEmpty()
        simple in androidxPreviewAnnotations || simple.endsWith("Preview")
    }
}
