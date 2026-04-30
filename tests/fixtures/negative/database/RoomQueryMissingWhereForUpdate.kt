package test

annotation class Dao
annotation class Query(val value: String)

@Dao
interface UserDao {
    @Query("DELETE FROM users WHERE id = :id")
    fun delete(id: Long): Int

    @Query("UPDATE users SET name = :name WHERE id = :id")
    fun rename(id: Long, name: String): Int

    @Query("DELETE FROM users")
    fun deleteAll(): Int

    @Query("DELETE FROM users")
    fun clearAllUsers(): Int

    @Query("SELECT * FROM users")
    fun all(): List<Int>
}
