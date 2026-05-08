package test

annotation class Dao

@Dao
interface UserDao {
    fun loadUsers(): List<String>
}
