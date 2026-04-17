package test
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect

@Composable
fun Screen(vm: VM) {
    LaunchedEffect(Unit) { vm.tracker.seen = true }
    Content(vm.state)
}
