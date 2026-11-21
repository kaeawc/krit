package stubs

import androidx.compose.ui.test.SemanticsNodeInteraction
import androidx.compose.ui.test.SemanticsNodeInteractionsProvider
import androidx.compose.ui.test.assertDoesNotExist
import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.onNodeWithTag

fun composeNodeSmoke(node: SemanticsNodeInteraction) {
    node.assertIsDisplayed()
    node.assertDoesNotExist()
}

fun composeProviderSmoke(provider: SemanticsNodeInteractionsProvider) {
    provider.onNodeWithTag("ready").assertIsDisplayed()
}
