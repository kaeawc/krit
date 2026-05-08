package test

annotation class Dao
annotation class Query(val value: String)

@Dao
interface UserDao {
    @Query("SELECT * FROM users WHERE name LIKE '%' || :q || '%' ESCAPE '\\'")
    fun search(q: String): List<Int>

    @Query("SELECT * FROM users WHERE name LIKE :q || '%'")
    fun searchPrefix(q: String): List<Int>

    @Query("SELECT * FROM users WHERE name = :q")
    fun exact(q: String): List<Int>

    @Query("SELECT * FROM users WHERE id = :id")
    fun byId(id: Long): List<Int>
}
