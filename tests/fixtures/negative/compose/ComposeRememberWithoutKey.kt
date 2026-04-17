package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Chart(dataset: List<Int>) {
    val series = remember(dataset) { buildSeries(dataset) }
    Render(series)
}
