package test

import androidx.room.Room

class Context

class AppDb {
    fun userDao(): UserDao = UserDao()
}

class UserDao {
    fun all(): List<User> = emptyList()
}

class User

class UserRepository(private val context: Context) {
    fun loadUsers(): List<User> {
        val db = Room.databaseBuilder(context, AppDb::class.java, "app.db").build()
        return db.userDao().all()
    }
}
