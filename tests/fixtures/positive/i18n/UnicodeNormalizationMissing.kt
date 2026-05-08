package test

data class User(val name: String)

fun searchUsers(users: List<User>, query: String): List<User> =
    users.filter { it.name.contains(query, ignoreCase = true) }

fun findUser(users: List<User>, query: String): User? =
    users.firstOrNull { it.name.contains(query) }
