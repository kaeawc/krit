package test

import kotlinx.coroutines.flow.Flow
import androidx.paging.PagingSource

annotation class Dao
annotation class Query(val value: String)

@Dao
interface UserDao {
    @Query("SELECT * FROM users LIMIT 50")
    suspend fun bounded(): List<User>

    @Query("SELECT * FROM users")
    fun observe(): Flow<List<User>>

    @Query("SELECT * FROM users")
    fun paged(): PagingSource<Int, User>

    @Query("SELECT id, name FROM users")
    suspend fun projected(): List<User>

    @Query("SELECT count(*) FROM users")
    suspend fun count(): Int
}

class User
