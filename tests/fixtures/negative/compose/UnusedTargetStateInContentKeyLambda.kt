package fixtures.compose

import androidx.compose.animation.AnimatedContent
import androidx.compose.material.Text
import androidx.compose.runtime.Composable

@Composable
fun Counter(count: Int) {
    AnimatedContent(targetState = count, contentKey = { it / 10 }) { Text("$it") }
    AnimatedContent(targetState = count, contentKey = { state -> state % 2 }) { Text("$it") }
}
