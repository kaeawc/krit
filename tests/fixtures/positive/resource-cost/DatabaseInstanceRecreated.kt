package test

object Room {
    fun databaseBuilder(context: Context, klass: Class<AppDb>, name: String): Builder = Builder()
}

class Context

class Builder {
    fun build(): AppDb = AppDb()
}

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
