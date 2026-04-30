package test

annotation class Dao
annotation class Query(val value: String)

@Dao
interface UserDao {
    @Query("DELETE FROM users")
    fun delete(): Int

    @Query("UPDATE users SET name = :name")
    fun renameAll(name: String): Int
}
