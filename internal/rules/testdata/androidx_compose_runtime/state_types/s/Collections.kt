package s

import androidx.compose.runtime.Composable
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import java.util.LinkedList

fun collections(list: MutableList<Int>, ro: List<Int>, arr: ArrayList<String>) {
    val a = mutableStateOf(mutableListOf(1))
    val b = mutableStateOf(mutableSetOf(1))
    val c = mutableStateOf(mutableMapOf(1 to 1))
    val d = mutableStateOf(list)
    val e = mutableStateOf(ro)
    val f = mutableStateOf(arr)
    val g = mutableStateOf(ArrayList<Int>())
    val h = mutableStateOf(HashMap<Int, Int>())
    val i = mutableStateOf(java.util.HashSet<Int>())
    val j = mutableStateOf(LinkedList<Int>())
    val k = mutableStateOf<List<Int>>(mutableListOf())
    val l: MutableState<List<Int>> = mutableStateOf(mutableListOf())
    val m = mutableStateOf<MutableList<Int>>(mutableListOf())
    val n = mutableStateOf(listOf(1))
    val o = mutableStateOf(ro.toMutableList())
    val p = mutableStateOf(emptyList<Int>())
    val q = mutableStateOf(Custom())
    val r: MutableState<MutableMap<String, Int>> = mutableStateOf(mutableMapOf())
    val t = mutableStateOf(arrayListOf(1))
}

class Custom : ArrayList<Int>()

@Composable
fun InComposition() {
    val s = remember { mutableStateOf(mutableListOf<Int>()) }
}
