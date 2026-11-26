package fixtures.compose

import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.lifecycle.LifecycleOwner
import androidx.lifecycle.compose.currentStateAsState

@Composable
fun Status(owner: LifecycleOwner) {
    val state by owner.lifecycle.currentStateAsState()
    Text(state.name)
    LaunchedEffect(owner) { println(owner.lifecycle.currentState) }
}

class Machine {
    var currentState = 0
}

@Composable
fun NotLifecycle(machine: Machine) {
    Text("${machine.currentState}")
}
