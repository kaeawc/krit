package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.annotation.DoNotRetain
import androidx.compose.runtime.retain.retain

@DoNotRetain("Holds a connection that is closed with the screen")
class Connection

@Composable
fun Screen() {
    val connection = retain { Connection() }
}
