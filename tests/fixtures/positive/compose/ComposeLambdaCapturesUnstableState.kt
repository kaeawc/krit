package test

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable

@Composable
fun UserList(vm: Vm, users: List<User>) {
	LazyColumn {
		items(users) { user ->
			Button(onClick = { vm.select(user) }) {
				Text(user.name)
			}
		}
	}
}

class Vm {
	fun select(user: User) {}
}

data class User(val name: String)
