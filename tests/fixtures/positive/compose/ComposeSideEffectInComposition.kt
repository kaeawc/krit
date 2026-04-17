package test
import androidx.compose.runtime.Composable

@Composable
fun Screen(vm: VM) {
    vm.tracker.seen = true
    Content(vm.state)
}
