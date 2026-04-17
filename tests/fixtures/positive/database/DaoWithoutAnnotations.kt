package test

annotation class Dao
annotation class Query(val value: String)

data class User(val id: Int)

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun all(): List<User>

    fun helper(): Int = 0
}
