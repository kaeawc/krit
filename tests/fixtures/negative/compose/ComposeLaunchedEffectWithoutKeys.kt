package fixtures.negative.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

@Composable
fun ComposeLaunchedEffectWithoutKeysNegative(userId: String) {
    LaunchedEffect(userId) {
        fetch(userId)
    }
}

private fun fetch(userId: String) {
    println(userId)
}
