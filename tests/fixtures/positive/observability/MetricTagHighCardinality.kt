package test

interface Registry {
    fun counter(name: String, vararg tags: String): Counter
}

interface Counter {
    fun increment()
}

data class User(val id: String)

fun handle(registry: Registry, user: User) {
    registry.counter("events", "user_id", user.id).increment()
}
