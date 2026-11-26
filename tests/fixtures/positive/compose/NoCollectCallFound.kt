package fixtures.compose

import androidx.activity.compose.PredictiveBackHandler
import androidx.compose.runtime.Composable

@Composable
fun Back(onDone: () -> Unit) {
    PredictiveBackHandler { progress -> onDone() }
}
