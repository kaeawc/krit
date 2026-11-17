// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 29, 37, 45
// A plain val local computed from a parameter changes with it, so a keyless remember that reads the local goes
// stale too and the key list names the local. Locals that hold remembered state, vars, delegated locals,
// destructured components, and locals excused like a parameter are not reported.
package test

import androidx.compose.runtime.Composable
import androidx.compose.runtime.key
import androidx.compose.runtime.remember

fun use(x: Any?) {}

@Composable
fun Wrapper(content: @Composable () -> Unit) {
    content()
}

@Composable
fun DerivedAlone(data: List<Int>) {
    val total = data.size
    val v = <!ComposeRememberWithoutKey!>remember<!> { total + 1 }
    use(v)
}

@Composable
fun DerivedAndParameter(data: List<Int>, scale: Int) {
    val total = data.size
    val v = <!ComposeRememberWithoutKey!>remember<!> { total * scale }
    use(v)
}

@Composable
fun DerivedChain(data: List<Int>) {
    val first = data.first()
    val doubled = first * 2
    val v = <!ComposeRememberWithoutKey!>remember<!> { doubled }
    use(v)
}

@Composable
fun DerivedInContentLambda(data: List<Int>) {
    Wrapper {
        val total = data.size
        val v = <!ComposeRememberWithoutKey!>remember<!> { total }
        use(v)
    }
}

@Composable
fun NotCopies(data: List<Int>, pair: Pair<Int, Int>) {
    val held = remember(data) { data.size }
    val a = remember { held + 1 }
    var counter = data.size
    counter += 1
    val b = remember { counter }
    val lazyTotal by lazy { data.size }
    val c = remember { lazyTotal }
    val plain = 5
    val d = remember { plain }
    val (first, second) = pair
    val e = remember { first + second }
    use(a)
    use(b)
    use(c)
    use(d)
    use(e)
}

@Composable
fun ExcusedLikeParameters(data: List<Int>) {
    val total = data.size
    key(total) {
        val v = remember { total }
        use(v)
    }
    val initialTotal = data.size
    val w = remember { initialTotal }
    use(w)
    val size = data.size
    var x = remember { size }
    if (x != size) x = size
    use(x)
}
