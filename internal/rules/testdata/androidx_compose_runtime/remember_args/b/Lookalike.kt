package b

import androidx.compose.runtime.Composable

fun remember(block: () -> Unit) = block()
fun produceUnit() {}

class Store {
    fun <T> produceState(initial: T, block: () -> Unit): T = initial
    fun <T> rememberSaveable(vararg inputs: Any?, init: () -> T): T = init()
}

object PlainSaver

@Composable
fun Lookalikes(store: Store) {
    remember { produceUnit() }
    store.produceState(0) { }
    store.rememberSaveable(PlainSaver) { 1 }
    // remember { produceUnit() }
    val s = "remember(produceUnit()) { 1 }"
}
