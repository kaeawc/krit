package test

annotation class Dao
annotation class Query(val value: String)
annotation class Transaction

data class User(val id: Long)
data class Post(val id: Long)
data class UserWithPosts(val user: User, val posts: List<Post>)

@Dao
interface UserDao {
    @Query("SELECT * FROM users WHERE id = :id")
    fun getUserBlocking(id: Long): User

    @Query("SELECT * FROM posts WHERE user_id = :id")
    fun getPostsBlocking(id: Long): List<Post>

    @Query("SELECT * FROM users WHERE id = :id")
    suspend fun getUser(id: Long): User

    // Blocking @Transaction calling blocking queries — Room does not auto-wrap.
    @Transaction
    fun loadBlocking(id: Long): UserWithPosts =
        UserWithPosts(getUserBlocking(id), getPostsBlocking(id))

    // Suspend @Transaction with no query calls — nothing to double-wrap.
    @Transaction
    suspend fun touch(id: Long) {
        println("id=$id")
    }

    // Suspend @Query without @Transaction — Room handles it directly.
    @Query("SELECT * FROM users WHERE id = :id")
    suspend fun fetch(id: Long): User
}

class NotADao {
    @Transaction
    suspend fun load(id: Long): User {
        return getUser(id)
    }

    suspend fun getUser(id: Long): User = User(id)
}
