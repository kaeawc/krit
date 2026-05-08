package test

import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material3.Button
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.remember

@Composable
fun UserList(vm: Vm, users: List<User>) {
	LazyColumn {
		items(users) { user ->
			val onClick = remember(user) { { vm.select(user) } }
			Button(onClick = onClick) {
				Text(user.name)
			}
		}
	}
}

@Composable
fun UserListPropertyRead(vm: Vm, users: List<User>) {
	LazyColumn {
		items(users) { user ->
			Button(onClick = { vm.selectById(user.id) }) {
				Text(user.name)
			}
		}
	}
}

class Vm {
	fun select(user: User) {}
	fun selectById(id: String) {}
}

data class User(val id: String, val name: String)
