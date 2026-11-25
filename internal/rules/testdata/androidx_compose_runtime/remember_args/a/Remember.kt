package a

import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.ReusableContent
import androidx.compose.runtime.key
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.produceState
import androidx.compose.runtime.remember

class TestState {
    fun update(value: Int) {}
    fun updated(value: Int): Boolean = true
}

fun produceUnit() {}
fun produceInt(): Int = 1
val unitProperty: Unit = Unit

@Composable
fun Returns(state: TestState, unitLambda: () -> Unit, k: Any) {
    remember { state.update(5) }
    remember(k) { state.update(5) }
    remember(k, k, k, k) { state.update(5) }
    remember<Unit> { state.updated(5) }
    remember(unitLambda)
    remember(k, calculation = unitLambda)
    remember { state.updated(5) }
    remember { }
    val x = remember { produceUnit() }
    val y: Any = remember { produceUnit() }
    remember { Unit }
    remember { var i = 0; i = 2 }
    remember { println("x") }
    remember { produceInt() }
}

@Composable
fun Keys(flag: Boolean) {
    remember(unitProperty) { 1 }
    remember(produceUnit()) { 1 }
    remember(42, produceUnit()) { 1 }
    remember(1, 2, 3, produceUnit()) { 1 }
    remember(Unit) { 1 }
    remember(kotlin.Unit) { 1 }
    remember(if (flag) produceUnit() else produceUnit()) { 1 }
    remember(if (flag) produceUnit() else 42) { 1 }
    LaunchedEffect(produceUnit()) { }
    LaunchedEffect(1, produceUnit()) { }
    DisposableEffect(produceUnit()) { onDispose { } }
    val p = produceState(0, produceUnit()) { value = 1 }
    ReusableContent(produceUnit()) { }
    key(produceUnit()) { }
    remember(key1 = produceUnit()) { 1 }
    remember(produceInt()) { 1 }
}

@Composable
fun Produce(other: MutableState<Int>, s: String) {
    produceState(0) { val foo = value }
    produceState(0) { value = 1 }
    produceState(0) { this.value = 1 }
    produceState(0) { value += 1 }
    produceState(0) { value++ }
    produceState(0) { awaitDispose { } }
    produceState(0) { helper() }
    produceState(0) { other.value = 2 }
    produceState(0, producer = { value = 3 })
    produceState(0) { println(s) }
    produceState(0) { launchNested { value = 4 } }
}

fun <T> MutableState<T>.helper() {}
fun launchNested(block: () -> Unit) = block()
