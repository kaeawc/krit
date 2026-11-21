package androidx.compose.ui.test.junit4

import androidx.compose.runtime.Composable
import androidx.compose.ui.test.SemanticsNodeInteractionsProvider

interface ComposeTestRule : SemanticsNodeInteractionsProvider {
    fun waitForIdle()
    fun waitUntilNodeCount(matcher: androidx.compose.ui.test.SemanticsMatcher, count: Int)
}

interface ComposeContentTestRule : ComposeTestRule {
    fun setContent(composable: @Composable () -> Unit)
}
