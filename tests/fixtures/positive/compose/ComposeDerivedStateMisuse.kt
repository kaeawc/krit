package fixtures.positive.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember

@Composable
fun ComposeDerivedStateMisusePositive() {
    val count by remember { mutableStateOf(0) }
    val isPositive by remember { derivedStateOf { count > 0 } }
    println(isPositive)
}
