package test

annotation class Dao
annotation class Query(val value: String)
annotation class Insert
annotation class Update
annotation class Delete
annotation class Transaction

data class User(val id: Int)

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun all(): List<User>

    @Insert
    fun insert(user: User)

    @Update
    fun update(user: User)

    @Delete
    fun delete(user: User)

    @Transaction
    fun refresh()

    companion object {
        fun helper(): Int = 0
    }
}
