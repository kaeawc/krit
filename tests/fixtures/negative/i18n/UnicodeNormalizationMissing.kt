package test

import java.text.Normalizer

data class User(val name: String)

fun searchUsers(users: List<User>, query: String): List<User> {
    val normalized = Normalizer.normalize(query, Normalizer.Form.NFC)
    return users.filter {
        Normalizer.normalize(it.name, Normalizer.Form.NFC)
            .contains(normalized, ignoreCase = true)
    }
}

// Outside a search/find function — not gated.
fun greet(name: String, suffix: String): Boolean =
    name.contains(suffix, ignoreCase = true)
