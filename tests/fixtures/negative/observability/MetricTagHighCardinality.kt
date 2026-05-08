package test

interface Registry {
    fun counter(name: String, vararg tags: String): Counter
}

interface Counter {
    fun increment()
}

data class User(val tier: String)

fun handle(registry: Registry, user: User) {
    registry.counter("events", "tier", user.tier).increment()
}
