package architecture

data class User(val id: Long, val name: String)

internal class InternalUserService {
    fun get(id: Long): User = User(id, "user-$id")

    fun save(user: User) {
        println(user)
    }
}

class UserService(private val impl: InternalUserService) {
    fun get(id: Long): User = impl.get(id)

    fun save(user: User) {
        impl.save(user)
    }
}
