package fixtures.negative.compose

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun ComposeRememberInListNegative(users: List<User>) {
    LazyColumn {
        items(users, key = { it.id }) { user ->
            val state = remember(user) { expensiveBuilder(user) }
            println(state)
        }
    }
}

private fun expensiveBuilder(user: User): String = user.name.uppercase()

data class User(val id: String, val name: String)
