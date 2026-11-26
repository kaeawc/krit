package fixtures.compose

import androidx.compose.animation.AnimatedContent
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Counter(count: Int) {
    AnimatedContent(targetState = count, contentKey = { 0 }) { Text("$it") }
}
