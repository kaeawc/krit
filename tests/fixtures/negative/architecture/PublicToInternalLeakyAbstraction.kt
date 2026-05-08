package architecture

data class User(val id: Long, val name: String)
data class PublicUser(val id: Long, val displayName: String)

internal class InternalUserService {
    fun get(id: Long): User = User(id, " user-$id ")
}

class UserProfileService(private val impl: InternalUserService) {
    fun getProfile(id: Long): PublicUser {
        require(id > 0)

        val user = impl.get(id)
        return PublicUser(user.id, user.name.trim().uppercase())
    }
}
