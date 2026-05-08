package test

annotation class Dao

@Dao
abstract class UserDao {
    abstract fun loadUsers(): List<String>
}
