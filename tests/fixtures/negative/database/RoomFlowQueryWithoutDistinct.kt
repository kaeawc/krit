package test

annotation class Dao
annotation class Query(val value: String)

class Flow<T>

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun observeUsers(): Flow<List<Int>>

    @Query("SELECT id FROM users WHERE id = :id")
    suspend fun byId(id: Long): Int
}

class UserRepository(private val dao: UserDao) {
    suspend fun render() {
        dao.observeUsers().distinctUntilChanged().collect { /* ... */ }
    }

    suspend fun renderMapped() {
        dao.observeUsers().map { it.size }.distinctUntilChanged().collect { /* ... */ }
    }

    suspend fun renderById() {
        dao.byId(1)
    }
}
