package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Chart(dataset: List<Int>) {
    val series = remember { buildSeries(dataset) }
    Render(series)
}
