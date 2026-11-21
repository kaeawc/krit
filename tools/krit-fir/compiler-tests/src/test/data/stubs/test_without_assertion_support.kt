package test.testwithoutassertion.support

import androidx.compose.ui.test.assertIsDisplayed
import androidx.compose.ui.test.junit4.ComposeContentTestRule
import androidx.compose.ui.test.onNodeWithTag

open class BenchmarkBase {
    val benchmarkRule: androidx.benchmark.junit4.BenchmarkRule = TODO()
}

fun ComposeContentTestRule.verifyLoginShown() { onNodeWithTag("login").assertIsDisplayed() }
fun ComposeContentTestRule.displayLogin() { onNodeWithTag("login").assertIsDisplayed() }
class Repository {
    fun load() = Unit
}
class CustomSubject : com.google.common.truth.Subject() {
    fun matchesValue(value: Int) = Unit
}
interface Listener {
    var onChange: (Int) -> Unit
}
