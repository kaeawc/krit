package fixtures.positive.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

@Composable
fun ComposeLaunchedEffectWithoutKeysPositive(userId: String) {
    LaunchedEffect(Unit) {
        fetch(userId)
    }
}

private fun fetch(userId: String) {
    println(userId)
}
