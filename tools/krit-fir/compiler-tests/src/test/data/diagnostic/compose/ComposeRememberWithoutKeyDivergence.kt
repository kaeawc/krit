// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 34, 37, 47, 56
// Where the checker reads resolution and Go reads names and syntax.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.movableContentOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.remember as rem

fun use(x: Any?) {}

// A calculation passed in parentheses or by name is still the calculation. Go only reads a trailing lambda, so
// these are FIR-only true positives.
@Composable
fun NonTrailing(data: List<Int>) {
    val a = <!ComposeRememberWithoutKey!>remember<!>({ data.size })
    val b = <!ComposeRememberWithoutKey!>remember<!>(calculation = { data.size })
    use(a)
    use(b)
}

// An import alias still resolves to Compose's remember. Go matches the written name and misses it.
@Composable
fun Aliased(data: List<Int>) {
    val v = <!ComposeRememberWithoutKey!>rem<!> { data.size }
    use(v)
}

// A lambda parameter or a local declared outside the calculation with the parameter's name is a different symbol.
// Go only checks shadowing inside the calculation, so it reports both.
@Composable
fun ShadowedOutside(items: List<Int>, data: List<Int>) {
    items.forEach { items -> use(remember { items }) }
    val v = run {
        val data = listOf(1)
        remember { data.size }
    }
    use(v)
}

// The keyless branch of a when whose other branch keys the remember is the fallback. Go only reads if/else.
@Composable
fun WhenFallback(token: String?) {
    val v = when {
        token != null -> remember(token) { token.length }
        else -> remember { token.hashCode() }
    }
    use(v)
}

// A composable lambda handed to movableContentOf directly is wrapped like `movableContentOf { content() }`. Go only
// treats a read inside the wrapper's lambda as tracked and reports this one.
@Composable
fun MovableDirect(content: @Composable () -> Unit) {
    val movable = remember { movableContentOf(content) }
    use(movable)
}
