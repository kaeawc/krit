// Smoke: state delegates (getValue/setValue), destructuring, effects, and
// composition locals, written the way real composables write them.
package stubs

import androidx.compose.runtime.Composable
import androidx.compose.runtime.CompositionLocalProvider
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.Immutable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.MutableState
import androidx.compose.runtime.SideEffect
import androidx.compose.runtime.Stable
import androidx.compose.runtime.State
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableFloatStateOf
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.rememberUpdatedState
import androidx.compose.runtime.setValue
import androidx.compose.runtime.snapshotFlow
import androidx.compose.runtime.staticCompositionLocalOf
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.launch

@Stable
class StableHolder(val value: Int)

@Immutable
data class ImmutableHolder(val value: Int)

val LocalSmokeValue = staticCompositionLocalOf { 0 }

@Composable
fun Counter(label: String, onDone: () -> Unit, flow: StateFlow<Int>, events: Flow<String>) {
    var count by remember { mutableStateOf(0) }
    var intCount by remember { mutableIntStateOf(0) }
    var floatCount by remember { mutableFloatStateOf(0f) }
    val (text, setText) = remember { mutableStateOf("") }
    val holder: MutableState<Boolean> = remember { mutableStateOf(false) }
    val keyed = remember(label) { label.length }
    val twoKeys = remember(label, onDone) { label.length + 1 }
    val derived by remember { derivedStateOf { count * 2 } }
    val latestOnDone by rememberUpdatedState(onDone)
    val collected: State<Int> = flow.collectAsState()
    val state by flow.collectAsState()
    val event by events.collectAsState(initial = "")
    val scope = rememberCoroutineScope()
    LaunchedEffect(label) {
        delay(1)
        latestOnDone()
        snapshotFlow { count }.collect { println(it) }
    }
    LaunchedEffect(Unit, label) {}
    DisposableEffect(label) {
        val job = scope.launch {}
        onDispose { job.cancel() }
    }
    SideEffect { holder.value = true }
    CompositionLocalProvider(LocalSmokeValue provides 1) {
        val current = LocalSmokeValue.current
        count = intCount + floatCount.toInt() + keyed + twoKeys + derived + state + current + collected.value
    }
    intCount += 1
    floatCount = 1f
    setText(text + event)
}
