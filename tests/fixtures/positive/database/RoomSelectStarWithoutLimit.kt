package test

annotation class Dao
annotation class Query(val value: String)

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    suspend fun all(): List<User>

    @Query("SELECT * FROM users WHERE active = 1")
    fun active(): List<User>
}

class User
