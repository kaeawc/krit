package fixtures.compose

import androidx.compose.animation.AnimatedContent
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Counter(count: Int) {
    AnimatedContent(count) { Text("$it") }
    AnimatedContent(targetState = count) { target -> Text("Count: $target") }
    AnimatedContent(count, label = "pair") { (value) -> Text("destructured parameters are skipped") }
}
