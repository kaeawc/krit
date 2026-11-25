package i

import androidx.compose.runtime.Composable
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.onEach
import kotlinx.coroutines.launch

@Composable
fun Collections(scope: CoroutineScope, list: List<Int>, arr: Array<Int>, map: Map<Int, Int>, seq: Sequence<Int>, flow: Flow<Int>, any: Iterable<Int>) {
    listOf(1).forEach { scope.launch {} }
    list.forEachIndexed { _, _ -> scope.launch {} }
    list.any { scope.launch {}; true }
    list.all { scope.launch {}; true }
    list.none { scope.launch {}; true }
    list.count { scope.launch {}; true }
    list.find { scope.launch {}; true }
    list.sumOf { scope.launch {}; 1 }
    list.map { scope.launch {} }
    list.filter { scope.launch {}; true }
    list.filterNot { scope.launch {}; true }
    list.mapNotNull { scope.launch {} }
    list.mapIndexed { _, _ -> scope.launch {} }
    list.flatMap { scope.launch {}; listOf(1) }
    list.onEach { scope.launch {} }
    list.first { scope.launch {}; true }
    list.fold(0) { acc, _ -> scope.launch {}; acc }
    arr.map { scope.launch {} }
    map.forEach { _, _ -> scope.launch {} }
    map.map { scope.launch {} }
    any.map { scope.launch {} }
    seq.forEach { scope.launch {} }
    seq.map { scope.launch {} }
    seq.filter { scope.launch {}; true }
    flow.map { scope.launch {} }
    flow.filter { scope.launch {}; true }
    flow.onEach { scope.launch {} }
    unknown().map { scope.launch {} }
}

fun unknown(): List<Int> = emptyList()
