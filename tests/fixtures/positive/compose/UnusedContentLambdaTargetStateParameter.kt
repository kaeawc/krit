package fixtures.compose

import androidx.compose.animation.AnimatedContent
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Counter(count: Int) {
    AnimatedContent(count) { Text("Count") }
    AnimatedContent(targetState = count) { target -> Text("Count") }
}
