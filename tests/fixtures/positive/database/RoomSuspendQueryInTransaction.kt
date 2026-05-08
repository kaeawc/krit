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
    suspend fun getUser(id: Long): User

    @Query("SELECT * FROM posts WHERE user_id = :id")
    suspend fun getPosts(id: Long): List<Post>

    @Transaction
    suspend fun load(id: Long): UserWithPosts {
        val user = getUser(id)
        return UserWithPosts(user, getPosts(id))
    }
}
