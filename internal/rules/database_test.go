package rules_test

import (
	"path/filepath"
	"strings"
	"testing"

	v2rules "github.com/kaeawc/krit/internal/rules/v2"
	"github.com/kaeawc/krit/internal/scanner"
)

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

func TestRoomConflictStrategyReplaceOnFk(t *testing.T) {
	rule := buildRuleIndex()["RoomConflictStrategyReplaceOnFk"]
	if rule == nil {
		t.Fatal("RoomConflictStrategyReplaceOnFk rule not registered")
	}
	if !rule.Needs.Has(v2rules.NeedsCrossFile) {
		t.Fatal("RoomConflictStrategyReplaceOnFk does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "database", "RoomConflictStrategyReplaceOnFk.kt")
	negativePath := filepath.Join(root, "negative", "database", "RoomConflictStrategyReplaceOnFk.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d", len(findings))
		}
		if !strings.Contains(findings[0].Message, "User") {
			t.Fatalf("expected finding to reference User entity, got %q", findings[0].Message)
		}
	})

	t.Run("negative fixture is clean", func(t *testing.T) {
		file, err := scanner.ParseFile(negativePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", negativePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})

	t.Run("cross-file entity declaration triggers", func(t *testing.T) {
		files := parseKotlinFiles(t,
			"Entities.kt", `package db

annotation class Entity(val foreignKeys: Array<ForeignKey> = [])
annotation class ForeignKey(val parent: kotlin.reflect.KClass<*>)

class Team(val id: Long)

@Entity(foreignKeys = [ForeignKey(parent = Team::class)])
class User(val id: Long)
`,
			"UserDao.kt", `package db

annotation class Insert(val onConflict: Int = 1)
annotation class Dao

object OnConflictStrategy { const val REPLACE = 1 }

@Dao
interface UserDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insert(user: User)
}
`,
		)
		findings := runCrossFileRule(rule, files)
		if len(findings) != 1 {
			t.Fatalf("expected 1 cross-file finding, got %d", len(findings))
		}
	})
}

func TestJdbcResultSetLeakedFromFunction_Positive(t *testing.T) {
	findings := runRuleByName(t, "JdbcResultSetLeakedFromFunction", `
package test

import java.sql.ResultSet

interface Statement {
    fun executeQuery(sql: String): ResultSet
}

fun query(stmt: Statement, sql: String): ResultSet =
    stmt.executeQuery(sql)
`)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for function returning ResultSet, got %d: %v", len(findings), findings)
	}
}

func TestJdbcResultSetLeakedFromFunction_Negative(t *testing.T) {
	findings := runRuleByName(t, "JdbcResultSetLeakedFromFunction", `
package test

import java.sql.ResultSet

interface Statement {
    fun executeQuery(sql: String): ResultSet
}

inline fun <R> ResultSet.use(block: (ResultSet) -> R): R = block(this)

fun <R> query(stmt: Statement, sql: String, block: (ResultSet) -> R): R =
    stmt.executeQuery(sql).use(block)
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when ResultSet is consumed via a block, got %v", findings)
	}
}

func TestEntityPrimaryKeyNotStable_Positive(t *testing.T) {
	findings := runRuleByName(t, "EntityPrimaryKeyNotStable", `
package test

annotation class Entity
annotation class PrimaryKey(val autoGenerate: Boolean = false)

@Entity
data class User(
    @PrimaryKey var id: Long = 0,
    val name: String,
)
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for @Entity @PrimaryKey var without autoGenerate = true")
	}
}

func TestEntityPrimaryKeyNotStable_NegativeAutoGenerate(t *testing.T) {
	findings := runRuleByName(t, "EntityPrimaryKeyNotStable", `
package test

annotation class Entity
annotation class PrimaryKey(val autoGenerate: Boolean = false)

@Entity
data class User(
    @PrimaryKey(autoGenerate = true) val id: Long = 0,
    val name: String,
)
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for autoGenerate = true val, got %v", findings)
	}
}

func TestEntityPrimaryKeyNotStable_NegativeVal(t *testing.T) {
	findings := runRuleByName(t, "EntityPrimaryKeyNotStable", `
package test

annotation class Entity
annotation class PrimaryKey(val autoGenerate: Boolean = false)

@Entity
data class User(
    @PrimaryKey val id: Long,
    val name: String,
)
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for val primary key, got %v", findings)
	}
}

func TestEntityPrimaryKeyNotStable_NegativeNotEntity(t *testing.T) {
	findings := runRuleByName(t, "EntityPrimaryKeyNotStable", `
package test

annotation class PrimaryKey(val autoGenerate: Boolean = false)

data class User(
    @PrimaryKey var id: Long = 0,
    val name: String,
)
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings outside @Entity, got %v", findings)
	}
}

func TestEntityPrimaryKeyNotStable_PositiveAutoGenerateFalse(t *testing.T) {
	findings := runRuleByName(t, "EntityPrimaryKeyNotStable", `
package test

annotation class Entity
annotation class PrimaryKey(val autoGenerate: Boolean = false)

@Entity
data class User(
    @PrimaryKey(autoGenerate = false) var id: Long = 0,
    val name: String,
)
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for explicit autoGenerate = false var")
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
