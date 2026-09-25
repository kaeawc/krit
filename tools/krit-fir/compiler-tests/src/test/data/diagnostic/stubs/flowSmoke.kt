// Smoke: suspend collect (member with a FlowCollector SAM), flow builders,
// operators with suspend lambdas, and hot-flow sharing.
package stubs

import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.ExperimentalCoroutinesApi
import kotlinx.coroutines.Job
import kotlinx.coroutines.channels.awaitClose
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.FlowCollector
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.flow.callbackFlow
import kotlinx.coroutines.flow.catch
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.flow.combine
import kotlinx.coroutines.flow.debounce
import kotlinx.coroutines.flow.distinctUntilChanged
import kotlinx.coroutines.flow.emptyFlow
import kotlinx.coroutines.flow.filter
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.flow.flatMapLatest
import kotlinx.coroutines.flow.flow
import kotlinx.coroutines.flow.flowOf
import kotlinx.coroutines.flow.flowOn
import kotlinx.coroutines.flow.launchIn
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.flow.onCompletion
import kotlinx.coroutines.flow.onEach
import kotlinx.coroutines.flow.onStart
import kotlinx.coroutines.flow.shareIn
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.flow.take
import kotlinx.coroutines.flow.toList

class FlowSmokeStore(private val scope: CoroutineScope) {
    private val _state = MutableStateFlow(0)
    val state: StateFlow<Int> = _state.asStateFlow()
    private val _events = MutableSharedFlow<String>(replay = 1, extraBufferCapacity = 8)
    val events: SharedFlow<String> = _events.asSharedFlow()

    suspend fun publish(value: Int) {
        _state.value = value
        _state.emit(value + 1)
        _events.emit("published")
        _events.tryEmit("again")
    }

    fun shared(source: Flow<Int>): StateFlow<Int> =
        source.stateIn(scope, SharingStarted.WhileSubscribed(5_000), initialValue = 0)

    fun replayed(source: Flow<Int>): SharedFlow<Int> = source.shareIn(scope, SharingStarted.Eagerly, replay = 1)

    fun observe(): Job = state.onEach { println(it) }.launchIn(scope)
}

suspend fun consume(source: Flow<Int>) {
    source.collect { value -> println(value) }
    source.collect(FlowCollector { value -> println(value) })
    source.collectLatest { println(it) }
    val first: Int = source.first()
    val all: List<Int> = source.take(2).toList()
    println("$first $all")
}

@OptIn(ExperimentalCoroutinesApi::class)
fun pipeline(source: Flow<Int>, other: Flow<String>): Flow<String> =
    source
        .map { it * 2 }
        .filter { it > 0 }
        .distinctUntilChanged()
        .debounce(100L)
        .onStart { emit(0) }
        .onCompletion { cause -> println(cause) }
        .catch { error -> println(error) }
        .flatMapLatest { value -> flowOf(value, value) }
        .combine(other) { number, text -> "$number$text" }
        .flowOn(Dispatchers.Default)

fun builders(): List<Flow<Int>> = listOf(
    flow {
        emit(1)
        emit(2)
    },
    flowOf(1, 2, 3),
    listOf(1, 2).asFlow(),
    emptyFlow(),
    callbackFlow {
        trySend(1)
        awaitClose { println("closed") }
    },
)
