package fixtures.positive.resourcecost

class DatabaseQueryActivity : android.app.Activity() {
    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        val db: android.database.sqlite.SQLiteDatabase = TODO()
        val cursor = db.rawQuery("SELECT * FROM users", null)
        cursor.close()
    }
}

annotation class Dao
annotation class Query(val value: String)

class User

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun users(): List<User>
}

class RoomQueryActivity(private val userDao: UserDao) : android.app.Activity() {
    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        userDao.users()
    }
}

class AppDatabase(val userQueries: UserQueries)

class UserQueries {
    fun selectAll(): SqlDelightQuery<User> = TODO()
}

class SqlDelightQuery<T> {
    fun executeAsList(): List<T> = TODO()
}

class SqlDelightQueryActivity(private val database: AppDatabase) : android.app.Activity() {
    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        database.userQueries.selectAll().executeAsList()
    }
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

class SignalDatabaseActivity : android.app.Activity() {
    override fun onCreate(savedInstanceState: android.os.Bundle?) {
        SignalDatabase.messages.getMessageRecord(1)
    }
}
