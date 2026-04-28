package fixtures.negative.resourcecost

class UserTable {
    fun loadUsers(db: android.database.sqlite.SQLiteDatabase) {
        val cursor = db.rawQuery("SELECT * FROM users", null)
        cursor.close()
    }

    suspend fun loadUsersSuspending(db: android.database.sqlite.SQLiteDatabase) {
        val cursor = db.rawQuery("SELECT * FROM users", null)
        cursor.close()
    }
}

annotation class Dao
annotation class Query(val value: String)

class User
interface Flow<T>

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun users(): List<User>

    @Query("SELECT * FROM users")
    suspend fun usersSuspending(): List<User>

    @Query("SELECT * FROM users")
    fun observeUsers(): Flow<User>
}

class RoomRepository(private val userDao: UserDao) {
    fun loadUsers(): List<User> = userDao.users()
}

class AppDatabase(val userQueries: UserQueries)

class UserQueries {
    fun selectAll(): SqlDelightQuery<User> = TODO()
}

class SqlDelightQuery<T> {
    fun executeAsList(): List<T> = TODO()
}

class SqlDelightRepository(private val database: AppDatabase) {
    fun loadUsers(): List<User> = database.userQueries.selectAll().executeAsList()
}

open class DatabaseTable {
    val readableDatabase: SignalSQLiteDatabase = TODO()
}

class SignalSQLiteDatabase {
    fun query(table: String, columns: Array<String>?, selection: String?, selectionArgs: Array<String>?, groupBy: String?, having: String?, orderBy: String?): Cursor = TODO()
}

class Cursor {
    fun close() = Unit
}

class MessageTable : DatabaseTable() {
    fun getMessageRecord(id: Long): Cursor {
        return readableDatabase.query("message", null, "_id = ?", arrayOf(id.toString()), null, null, null)
    }
}

object SignalDatabase {
    val messages: MessageTable = MessageTable()
}

class SignalDatabaseRepository {
    fun loadMessage(id: Long): Cursor {
        return SignalDatabase.messages.getMessageRecord(id)
    }
}
