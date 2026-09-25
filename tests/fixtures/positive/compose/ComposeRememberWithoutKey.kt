package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun Chart(dataset: List<Int>) {
    val series = remember { buildSeries(dataset) }
    Render(series)
}

private fun buildSeries(dataset: List<Int>): List<Int> = dataset.sorted()

@Composable
private fun Render(series: List<Int>) {}
