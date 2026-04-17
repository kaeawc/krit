package fixtures.negative.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember

@Composable
fun ComposeDerivedStateMisuseNegative() {
    val count by remember { mutableStateOf(0) }
    val bucket by remember { derivedStateOf { count / 10 } }
    println(bucket)
}
