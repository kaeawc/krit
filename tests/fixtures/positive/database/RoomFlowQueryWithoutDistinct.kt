package test

annotation class Dao
annotation class Query(val value: String)

class Flow<T>

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun observeUsers(): Flow<List<Int>>
}

class UserRepository(private val dao: UserDao) {
    suspend fun render() {
        dao.observeUsers().collect { /* ... */ }
    }
}
