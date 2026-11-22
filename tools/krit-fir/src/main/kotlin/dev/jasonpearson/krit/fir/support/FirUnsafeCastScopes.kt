package dev.jasonpearson.krit.fir.support

import com.intellij.lang.LighterASTNode
import org.jetbrains.kotlin.KtNodeTypes
import org.jetbrains.kotlin.KtSourceElement
import org.jetbrains.kotlin.lexer.KtTokens

fun isUnsafeCastScaffolding(source: KtSourceElement, options: Map<String, Any?>): Boolean {
    var node = source.treeStructure.getParent(source.lighterASTNode)
    while (node != null) {
        if (node.tokenType == KtNodeTypes.FUN) {
            val children = lightChildren(source, node)
            val name = children.firstOrNull { it.tokenType == KtTokens.IDENTIFIER }
                ?.let { lightText(source, it).removeSurrounding("`").lowercase() }.orEmpty()
            if (name.isEmpty()) {
                node = source.treeStructure.getParent(node)
                continue
            }
            val modifiers = children.firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST }
            val annotations = modifiers?.let { lightChildren(source, it) }
                ?.filter { it.tokenType == KtNodeTypes.ANNOTATION_ENTRY }.orEmpty()
            if (isScaffoldingName(name) || annotations.any { isPreviewAnnotation(source, it, options) }) return true
            return annotations.any { annotation ->
                isLocalMultipreview(source, annotationSimpleName(source, annotation))
            }
        }
        node = source.treeStructure.getParent(node)
    }
    return false
}

private fun isScaffoldingName(name: String): Boolean {
    val prefixes = listOf("preview", "sample", "fake", "mock", "stub", "fixture")
    val suffixes = listOf("preview", "sample", "fixture")
    return prefixes.any(name::startsWith) || suffixes.any(name::endsWith)
}

private fun isPreviewAnnotation(
    source: KtSourceElement,
    annotation: LighterASTNode,
    options: Map<String, Any?> = emptyMap(),
): Boolean {
    val name = annotationSimpleName(source, annotation)
    val builtins = setOf(
        "Preview", "PreviewLightDark", "PreviewFontScale", "PreviewScreenSizes", "PreviewDynamicColors",
    )
    if (name in builtins) return true
    val prefixes = when (val configured = options["customPreviewPrefixes"]) {
        is String -> listOf(configured)
        is List<*> -> configured.mapNotNull { item ->
            when (item) {
                is String -> item
                is Map<*, *> -> item["value"] as? String
                else -> null
            }
        }
        else -> emptyList()
    }.map(::trimGoSpace).filter(String::isNotEmpty)
    if (prefixes.any { prefix ->
        name in setOf(prefix, prefix + "Preview", prefix + "Previews")
    }) return true
    val wildcard = options["customPreviewWildcard"] as? Boolean ?: true
    return wildcard && (name.startsWith("Preview") || name.endsWith("Preview") || name.endsWith("Previews"))
}

private fun annotationSimpleName(source: KtSourceElement, annotation: LighterASTNode): String =
    lightText(source, annotation).substringAfter('@').substringBefore('(').substringAfterLast(':')
        .substringAfterLast('.').let(::trimGoSpace).removeSurrounding("`")

private fun isLocalMultipreview(source: KtSourceElement, annotationName: String): Boolean {
    if (annotationName.isEmpty()) return false
    val root = sourceRoot(source)
    val pending = ArrayDeque<LighterASTNode>()
    pending.add(root)
    while (pending.isNotEmpty()) {
        val node = pending.removeLast()
        if (node.tokenType == KtNodeTypes.CLASS && isPreviewAnnotationClass(source, node, annotationName)) return true
        pending.addAll(lightChildren(source, node))
    }
    return false
}

private fun isPreviewAnnotationClass(source: KtSourceElement, node: LighterASTNode, name: String): Boolean {
    val children = lightChildren(source, node)
    val identifier = children.firstOrNull { it.tokenType == KtTokens.IDENTIFIER } ?: return false
    if (lightText(source, identifier).removeSurrounding("`") != name) return false
    val modifiers = children.firstOrNull { it.tokenType == KtNodeTypes.MODIFIER_LIST } ?: return false
    val modifierChildren = lightChildren(source, modifiers)
    if (modifierChildren.none { it.tokenType == KtTokens.ANNOTATION_KEYWORD }) return false
    return modifierChildren.any {
        it.tokenType == KtNodeTypes.ANNOTATION_ENTRY && isPreviewAnnotation(source, it)
    }
}

private fun sourceRoot(source: KtSourceElement): LighterASTNode {
    var node = source.lighterASTNode
    while (true) node = source.treeStructure.getParent(node) ?: return node
}
