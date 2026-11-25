package s

import androidx.compose.runtime.Composable
import androidx.compose.runtime.IntState
import androidx.compose.runtime.MutableDoubleState
import androidx.compose.runtime.MutableIntState
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.State
import androidx.compose.runtime.asIntState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableLongStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.neverEqualPolicy
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.runtime.structuralEqualityPolicy

fun values(int: MutableIntState, read: IntState, dbl: MutableDoubleState, generic: State<Int>) {
    val a = int.value
    int.value = 42
    int.value += 1
    int.value++
    val b = read.value
    val c = dbl.value
    val d = generic.value
    val e = int.intValue
    val f = mutableLongStateOf(1L)
    f.value
    val g = mutableFloatStateOf(1f).value
    val h = generic.asIntState().value
}

@Composable
fun Remembered() {
    val count = remember { mutableIntStateOf(0) }
    count.value
    var delegated by remember { mutableIntStateOf(0) }
    delegated = 2
}

class Holder(val state: MutableIntState) {
    val other = mutableIntStateOf(1)
    fun touch() {
        state.value
        this.other.value
    }
}

fun creation(n: Int, nullable: Int?) {
    val a = mutableStateOf(1)
    val b = mutableStateOf<Int>(1)
    val c = mutableStateOf<kotlin.Long>(1L)
    val d = mutableStateOf(1f)
    val e = mutableStateOf(1.0)
    val f = mutableStateOf(-1)
    val g = mutableStateOf(1, structuralEqualityPolicy())
    val h = mutableStateOf(1, neverEqualPolicy())
    val i = mutableStateOf<Int?>(1)
    val j: MutableState<Int?> = mutableStateOf(1)
    val k: MutableState<Int> = mutableStateOf(1)
    var l by mutableStateOf(1)
    var m: Int? by mutableStateOf(1)
    val o = mutableStateOf(n)
    val p = mutableStateOf(nullable)
    val q = mutableStateOf("x")
    val r = mutableStateOf(1 as Number)
    val t = remember0 { mutableStateOf(1) }
    val u: MutableState<Any> = mutableStateOf(1)
    use(mutableStateOf(1))
}

inline fun <T> remember0(block: () -> T): T = block()
fun use(any: Any?) {}
