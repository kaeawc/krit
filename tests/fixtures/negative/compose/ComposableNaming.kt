package fixtures.compose

import androidx.compose.runtime.Composable

@Composable
fun Button() {}

@Composable
fun getCount(): Int = 1

fun notComposable() {}

interface Screen {
    @Composable
    fun Content()
}

class Home : Screen {
    @Composable
    override fun Content() {}
}

object Holder {
    @Composable
    operator fun invoke() {}
}

@Composable
fun Outer() {
    // Lint does not visit local functions.
    @Composable
    fun inner() {}
    inner()
}
