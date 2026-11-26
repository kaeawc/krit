package fixtures.compose

import androidx.activity.compose.PredictiveBackHandler
import androidx.compose.runtime.Composable
import kotlinx.coroutines.flow.collect

@Composable
fun Back(onDone: () -> Unit) {
    PredictiveBackHandler { progress ->
        progress.collect { }
        onDone()
    }
}
