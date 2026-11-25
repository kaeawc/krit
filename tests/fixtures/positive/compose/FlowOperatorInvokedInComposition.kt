package fixtures.compose

import androidx.compose.runtime.Composable
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

@Composable
fun Names(users: Flow<List<String>>) {
    val upper = users.map { names -> names.map { it.uppercase() } }
}
