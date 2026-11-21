package stubs

import androidx.compose.ui.test.junit4.ComposeContentTestRule
import androidx.compose.ui.test.junit4.ComposeTestRule
import androidx.compose.ui.test.onNodeWithTag

fun composeRuleSmoke(rule: ComposeContentTestRule) {
    val parent: ComposeTestRule = rule
    rule.setContent {}
    parent.waitForIdle()
    parent.onNodeWithTag("tag")
}
