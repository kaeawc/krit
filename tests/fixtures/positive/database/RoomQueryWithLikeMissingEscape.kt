package test

annotation class Dao
annotation class Query(val value: String)

@Dao
interface UserDao {
    @Query("SELECT * FROM users WHERE name LIKE :q")
    fun search(q: String): List<Int>

    @Query("SELECT * FROM users WHERE name LIKE :prefix")
    fun searchPrefix(prefix: String): List<Int>
}
