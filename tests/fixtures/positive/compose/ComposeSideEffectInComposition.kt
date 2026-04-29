package test
import androidx.compose.runtime.Composable

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
