package c

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.drop
import kotlinx.coroutines.flow.emptyFlow
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.onEach

fun <T> Flow<T>.customOperator(param: Boolean): Flow<T> = this

@Composable
fun <T> Flow<T>.customComposableOperator(): Flow<T> = this

class Repo {
    val items: Flow<List<Int>> = emptyFlow()
    val state = MutableStateFlow(0)
    val readOnly: StateFlow<Int> = state.asStateFlow()
}

@Composable
fun Operators(repo: Repo, list: List<Int>) {
    val emptyFlow: Flow<Unit> = emptyFlow()
    emptyFlow.map { true }.customOperator(true).drop(0)
    emptyFlow.customComposableOperator()
    repo.items.filter { it.isNotEmpty() }
    repo.state.onEach { }
    repo.state.asStateFlow()
    list.map { it }
    list.filter { it > 0 }
    remember { repo.items.map { it } }
    val derived = remember(repo) { repo.state.map { it + 1 } }
}

fun nonComposable(repo: Repo) {
    repo.items.map { it }
}

class TestFlow : StateFlow<Boolean> {
    override val replayCache: List<Boolean> get() = emptyList()
    override val value: Boolean get() = false
    override suspend fun collect(collector: kotlinx.coroutines.flow.FlowCollector<Boolean>): Nothing = throw Exception()
}

@Composable
fun Values(repo: Repo, flow: StateFlow<Int>, custom: TestFlow) {
    repo.state.value
    repo.readOnly.value
    flow.value
    custom.value
    repo.state.value = 2
    val local = MutableStateFlow(true)
    local.value
    val anon = object : StateFlow<Boolean> {
        override val replayCache: List<Boolean> get() = emptyList()
        override val value: Boolean get() = false
        override suspend fun collect(collector: kotlinx.coroutines.flow.FlowCollector<Boolean>): Nothing = throw Exception()
    }
    anon.value
    val onClick = { flow.value }
    remember { flow.value }
}

class NotFlow { val value = 1 }

@Composable
fun NotStateFlow(n: NotFlow, s: androidx.compose.runtime.State<Int>) {
    n.value
    s.value
}

fun nonComposableValue(flow: StateFlow<Int>) = flow.value

@Composable
fun SelfReference(items: Flow<Int>) {
    val items = items.map { it }
    items.filter { true }
}
