package androidx.compose.ui.test

class SemanticsNodeInteraction

class SemanticsMatcher

fun hasText(text: String): SemanticsMatcher = TODO()

interface SemanticsNodeInteractionsProvider {
    fun onNode(matcher: SemanticsMatcher, useUnmergedTree: Boolean = false): SemanticsNodeInteraction
}

fun SemanticsNodeInteractionsProvider.onNodeWithTag(
    testTag: String,
    useUnmergedTree: Boolean = false,
): SemanticsNodeInteraction = TODO()

fun SemanticsNodeInteraction.assertIsDisplayed(): SemanticsNodeInteraction = TODO()

fun SemanticsNodeInteraction.assertDoesNotExist() {
    TODO()
}
