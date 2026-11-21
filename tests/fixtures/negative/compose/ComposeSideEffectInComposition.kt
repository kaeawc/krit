package test
import androidx.compose.animation.core.MutableTransitionState
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.remember
import androidx.compose.runtime.getValue
import androidx.compose.runtime.rxjava3.subscribeAsState
import androidx.compose.ui.Modifier
import androidx.compose.ui.focus.onFocusChanged
import androidx.compose.ui.layout.Layout
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.viewinterop.AndroidView
import com.example.retained.produceRetainedState
import androidx.navigation.NavType
import androidx.navigation.navArgument
import androidx.compose.ui.graphics.graphicsLayer
import kotlin.math.max

class Tracker(var seen: Boolean = false)
class State(val enabled: Boolean = false)
class VM(val tracker: Tracker = Tracker(), val state: State = State(), var expanded: Boolean = false, var position: Float = 0f)
class Output(var enabled: Boolean = false)
class Player
class PlayerView(context: android.content.Context) : android.view.View(context) { var player: Player? = null }
class Input
class Repo { suspend fun load(): List<String> = emptyList() }
class Source(val events: io.reactivex.rxjava3.core.Observable<SourceEvent>, var expanded: Boolean = false)
class SourceEvent(val shouldExpand: Boolean, val items: List<String>)
@Composable fun Content(value: Any? = null, modifier: Modifier = Modifier) {}
fun dialog(route: String, arguments: List<androidx.navigation.NamedNavArgument>, content: @Composable () -> Unit) {}

@Composable
fun Screen(vm: VM) {
    LaunchedEffect(Unit) { vm.tracker.seen = true }
    Content(vm.state)
}

@Composable
fun RememberInitializer(state: State) {
    val output = remember {
        Output().apply {
            enabled = state.enabled
        }
    }
    Content(output)
}

@Composable
fun ModifierCallback(modifier: Modifier = Modifier) {
    var focused = false
    Content(
        modifier = modifier
            .graphicsLayer {
                alpha = 0.5f
            }
            .onFocusChanged { state -> focused = state.hasFocus }
            .semantics { contentDescription = "content" }
    )
}

@Composable
fun LocalCallback(vm: VM) {
    Controls(vm.state, { vm.expanded = true }, updatePosition = { vm.position = it })
}

@Composable
fun Controls(
    state: State,
    setExpanded: (Boolean) -> Unit = {},
    updatePosition: (Float) -> Unit = {}
) {
    Content(state)
}

annotation class LocalComposable

@LocalComposable
fun NotCompose(vm: VM) {
    vm.tracker.seen = true
}

@Composable
fun AndroidViewCallbacks(player: Player) {
    AndroidView(
        factory = { context ->
            PlayerView(context).apply {
                this.player = player
            }
        },
        update = { view ->
            view.player = player
        }
    )
    AndroidView(factory = ::PlayerView) {
        it.player = player
    }
}

@Composable
fun NavigationBuilder() {
    dialog(
        route = "route/{id}",
        arguments = listOf(navArgument("id") { type = NavType.IntType })
    ) { Content() }
}

@Composable
fun RxCallback(source: Source) {
    val items = source.events
        .map { event ->
            source.expanded = event.shouldExpand
            event.items
        }
        .subscribeAsState(initial = emptyList())
    Content(items)
}

@Composable
fun MutableTransitionTargetState(expanded: Boolean) {
    val expandedStates = remember { MutableTransitionState(false) }
    expandedStates.targetState = expanded
    Content(expandedStates.currentState || expandedStates.targetState)
}

@Composable
fun RememberedObjectSynchronization(input: Input) {
    val holder = remember { Holder(input) }
    holder.input = input
    Content(holder.input)
}

private class Holder(var input: Input)

data class PresenterState(val selectedIndex: Int, val eventSink: (Event) -> Unit)
sealed interface Event
data class Selected(val index: Int) : Event

@Composable
fun LocalConstructorCallback() {
    var selectedIndex = 0
    Content(
        PresenterState(selectedIndex) { event ->
            when (event) {
                is Selected -> selectedIndex = event.index
            }
        }
    )
}

@Composable
fun RetainedStateProducer(repo: Repo) {
    val items by produceRetainedState(initialValue = emptyList<String>()) {
        value = repo.load()
    }
    Content(items)
}

@Composable
fun LayoutMeasurePolicy(content: @Composable () -> Unit) {
    Layout(content = content) { measurables, constraints ->
        val placeables = measurables.map { it.measure(constraints) }
        var width = 0
        var height = 0
        placeables.forEach {
            width += it.width
            height = max(height, it.height)
        }
        layout(width, height) {
            var offset = 0
            placeables.forEach {
                it.placeRelative(offset, 0)
                offset += it.width
            }
        }
    }
}
