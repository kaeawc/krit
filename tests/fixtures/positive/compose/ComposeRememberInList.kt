package fixtures.positive.compose

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun ComposeRememberInListPositive(users: List<User>) {
    LazyColumn {
        items(users) { user ->
            val state = remember { expensiveBuilder(user) }
            println(state)
        }
    }
}

private fun expensiveBuilder(user: User): String = user.name.uppercase()

data class User(val id: String, val name: String)
