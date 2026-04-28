package rules_test

import "testing"

func TestDatabaseInstanceRecreated_Positive(t *testing.T) {
	findings := runRuleByName(t, "DatabaseInstanceRecreated", `
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
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for Room.databaseBuilder inside a regular function")
	}
}

func TestDatabaseInstanceRecreated_Negative(t *testing.T) {
	findings := runRuleByName(t, "DatabaseInstanceRecreated", `
	package test

	import androidx.room.Room

	annotation class Module
	annotation class Provides

	class Context

	fun appContext(): Context = Context()

	class AppDb

@Module
object DbModule {
    @Provides
    fun provideDb(context: Context): AppDb =
        Room.databaseBuilder(context, AppDb::class.java, "app.db").build()
}

class Holder {
    companion object {
        val db: AppDb = Room.databaseBuilder(appContext(), AppDb::class.java, "app.db").build()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for @Provides or companion object initialization, got %v", findings)
	}
}

func TestDatabaseInstanceRecreated_PositiveJava(t *testing.T) {
	findings := runRuleByNameOnJava(t, "DatabaseInstanceRecreated", `
package test;

import androidx.room.Room;

class Context {}
class AppDb {}

class UserRepository {
  private final Context context;

  UserRepository(Context context) {
    this.context = context;
  }

  AppDb open() {
    return Room.databaseBuilder(context, AppDb.class, "app.db").build();
  }
}
`)
	if len(findings) != 1 {
		t.Fatalf("expected finding for Java Room.databaseBuilder inside a regular method, got %d", len(findings))
	}
}

func TestDatabaseInstanceRecreated_NegativeJavaLocalRoomTypeName(t *testing.T) {
	findings := runRuleByNameOnJava(t, "DatabaseInstanceRecreated", `
package test;

class Context {}
class AppDb {}
class Builder {
  AppDb build() { return new AppDb(); }
}
class Room {
  static Builder databaseBuilder(Context context, Class<AppDb> klass, String name) { return new Builder(); }
}

class UserRepository {
  AppDb open(Context context) {
    return Room.databaseBuilder(context, AppDb.class, "app.db").build();
  }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no finding for Java local Room type without androidx.room import, got %d", len(findings))
	}
}

func TestDaoNotInterface_Positive(t *testing.T) {
	findings := runRuleByName(t, "DaoNotInterface", `
package test

annotation class Dao

@Dao
abstract class UserDao {
    abstract fun loadUsers(): List<String>
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for @Dao abstract class")
	}
}

func TestDaoNotInterface_Negative(t *testing.T) {
	findings := runRuleByName(t, "DaoNotInterface", `
package test

annotation class Dao

@Dao
interface UserDao {
    fun loadUsers(): List<String>
}`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for @Dao interface, got %v", findings)
	}
}

func TestDaoWithoutAnnotations_Positive(t *testing.T) {
	findings := runRuleByName(t, "DaoWithoutAnnotations", `
package test

annotation class Dao
annotation class Query(val value: String)

data class User(val id: Int)

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun all(): List<User>

    fun helper(): Int = 0
}`)
	if len(findings) == 0 {
		t.Fatal("expected finding for unannotated @Dao function")
	}
}

func TestDaoWithoutAnnotations_Negative(t *testing.T) {
	findings := runRuleByName(t, "DaoWithoutAnnotations", `
package test

annotation class Dao
annotation class Query(val value: String)
annotation class Insert
annotation class Update
annotation class Delete
annotation class Transaction

data class User(val id: Int)

@Dao
interface UserDao {
    @Query("SELECT * FROM users")
    fun all(): List<User>

    @Insert
    fun insert(user: User)

    @Update
    fun update(user: User)

    @Delete
    fun delete(user: User)

    @Transaction
    fun refresh()

    companion object {
        fun helper(): Int = 0
    }
}`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when all @Dao functions are annotated, got %v", findings)
	}
}

func TestJdbcPreparedStatementNotClosed_Positive(t *testing.T) {
	findings := runRuleByName(t, "JdbcPreparedStatementNotClosed", `
package test

interface Connection {
    fun prepareStatement(sql: String): PreparedStatement
}

interface PreparedStatement {
    fun executeQuery(): ResultSet
    fun close()
}

interface ResultSet

fun query(connection: Connection) {
    val stmt = connection.prepareStatement("SELECT 1")
    stmt.executeQuery()
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for PreparedStatement without use or close")
	}
}

func TestJdbcPreparedStatementNotClosed_Negative(t *testing.T) {
	findings := runRuleByName(t, "JdbcPreparedStatementNotClosed", `
package test

interface Connection {
    fun prepareStatement(sql: String): PreparedStatement
}

interface PreparedStatement {
    fun executeQuery(): ResultSet
    fun close()
}

inline fun <T : PreparedStatement, R> T.use(block: (T) -> R): R = block(this)

fun query(connection: Connection) {
    val stmt = connection.prepareStatement("SELECT 1")
    stmt.executeQuery()
    stmt.close()
}

fun querySafely(connection: Connection) {
    connection.prepareStatement("SELECT 1").use { stmt ->
        stmt.executeQuery()
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when PreparedStatement is closed or wrapped in use, got %v", findings)
	}
}
