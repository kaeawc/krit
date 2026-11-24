package fixtures.compose

import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

@Composable
fun Names(users: Flow<List<String>>, names: List<String>) {
    val upper = remember(users) { users.map { list -> list.size } }
    val sizes = names.map { it.length }
}

fun outsideComposition(users: Flow<List<String>>) = users.map { it.size }
