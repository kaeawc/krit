package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.annotation.DoNotRetain
import androidx.compose.runtime.remember
import androidx.compose.runtime.retain.retain

@DoNotRetain
class Connection

class Cache

@Composable
fun Screen() {
    val connection = remember { Connection() }
    val cache = retain { Cache() }
}
