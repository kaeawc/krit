package dihygiene

annotation class Singleton
annotation class Inject

class User

class MutableStateFlow<T>(initial: T)
class StateFlow<T>

@Singleton
class UserCache @Inject constructor() {
    private val _state = MutableStateFlow<User?>(null)
    val state: StateFlow<User?> = StateFlow()
    val readOnly: List<User> = emptyList()
}

// Not @Singleton; not flagged.
class Scratch {
    var value: Int = 0
    val items = mutableListOf<Int>()
}
