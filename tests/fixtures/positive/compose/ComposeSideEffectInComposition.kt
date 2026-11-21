package test
import androidx.compose.runtime.Composable

class Tracker(var seen: Boolean = false)
class State
class VM(val tracker: Tracker = Tracker(), val state: State = State())
@Composable fun Content(state: State) {}

@Composable
fun Screen(vm: VM) {
    vm.tracker.seen = true
    Content(vm.state)
}

fun graphicsLayer(block: () -> Unit) {
    block()
}

@Composable
fun LocalLookalikeStillRunsInComposition(vm: VM) {
    graphicsLayer {
        vm.tracker.seen = true
    }
}

fun Layout(block: () -> Unit) {
    block()
}

@Composable
fun LocalLayoutLookalikeStillRunsInComposition(vm: VM) {
    Layout {
        vm.tracker.seen = true
    }
}
