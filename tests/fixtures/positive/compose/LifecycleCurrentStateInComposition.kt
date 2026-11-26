package fixtures.compose

import androidx.compose.material.Text
import androidx.compose.runtime.Composable
import androidx.lifecycle.LifecycleOwner

@Composable
fun Status(owner: LifecycleOwner) {
    Text(owner.lifecycle.currentState.name)
}
