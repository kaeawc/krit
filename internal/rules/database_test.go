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

func TestRoomEntityChangedMigrationMissing(t *testing.T) {
	rule := buildRuleIndex()["RoomEntityChangedMigrationMissing"]
	if rule == nil {
		t.Fatal("RoomEntityChangedMigrationMissing rule not registered")
	}
	if !rule.Needs.Has(v2rules.NeedsCrossFile) {
		t.Fatal("RoomEntityChangedMigrationMissing does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "database", "RoomEntityChangedMigrationMissing.kt")
	negativePath := filepath.Join(root, "negative", "database", "RoomEntityChangedMigrationMissing.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "avatarUrl") {
			t.Fatalf("expected finding to reference avatarUrl, got %q", findings[0].Message)
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

	t.Run("no migrations means no findings", func(t *testing.T) {
		files := parseKotlinFiles(t,
			"User.kt", `package db

annotation class Entity(val tableName: String = "")
annotation class PrimaryKey

@Entity(tableName = "users")
data class User(
    @PrimaryKey val id: Long,
    val avatarUrl: String,
)
`,
		)
		findings := runCrossFileRule(rule, files)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings without migrations, got %d", len(findings))
		}
	})

	t.Run("cross-file migration satisfies entity", func(t *testing.T) {
		files := parseKotlinFiles(t,
			"User.kt", `package db

annotation class Entity(val tableName: String = "")
annotation class PrimaryKey

@Entity(tableName = "users")
data class User(
    @PrimaryKey val id: Long,
    val avatarUrl: String,
)
`,
			"Migration1to2.kt", `package db

abstract class Migration(val from: Int, val to: Int) {
    abstract fun migrate(db: SupportSQLiteDatabase)
}
interface SupportSQLiteDatabase { fun execSQL(sql: String) }

object Migration1to2 : Migration(1, 2) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE users ADD COLUMN id INTEGER")
        db.execSQL("ALTER TABLE users ADD COLUMN avatarUrl TEXT")
    }
}
`,
		)
		findings := runCrossFileRule(rule, files)
		if len(findings) != 0 {
			t.Fatalf("expected 0 findings, got %d: %v", len(findings), findings)
		}
	})
}

func TestRoomRelationWithoutIndex(t *testing.T) {
	rule := buildRuleIndex()["RoomRelationWithoutIndex"]
	if rule == nil {
		t.Fatal("RoomRelationWithoutIndex rule not registered")
	}
	if !rule.Needs.Has(v2rules.NeedsCrossFile) {
		t.Fatal("RoomRelationWithoutIndex does not declare NeedsCrossFile")
	}

	root := fixtureRoot(t)
	positivePath := filepath.Join(root, "positive", "database", "RoomRelationWithoutIndex.kt")
	negativePath := filepath.Join(root, "negative", "database", "RoomRelationWithoutIndex.kt")

	t.Run("positive fixture triggers", func(t *testing.T) {
		file, err := scanner.ParseFile(positivePath)
		if err != nil {
			t.Fatalf("ParseFile(%s): %v", positivePath, err)
		}
		findings := runCrossFileRule(rule, []*scanner.File{file})
		if len(findings) != 1 {
			t.Fatalf("expected 1 finding, got %d: %v", len(findings), findings)
		}
		if !strings.Contains(findings[0].Message, "userId") || !strings.Contains(findings[0].Message, "Post") {
			t.Fatalf("expected finding to reference userId and Post, got %q", findings[0].Message)
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

annotation class Entity(val indices: Array<Index> = [])
annotation class Index(vararg val value: String)
annotation class PrimaryKey

@Entity
class User(@PrimaryKey val id: Long)

@Entity
class Post(@PrimaryKey val id: Long, val userId: Long)
`,
			"Joins.kt", `package db

annotation class Embedded
annotation class Relation(val parentColumn: String, val entityColumn: String)

class UserWithPosts(
    @Embedded val user: User,
    @Relation(parentColumn = "id", entityColumn = "userId")
    val posts: List<Post>,
)
`,
		)
		findings := runCrossFileRule(rule, files)
		if len(findings) != 1 {
			t.Fatalf("expected 1 cross-file finding, got %d: %v", len(findings), findings)
		}
	})

	t.Run("explicit entity argument resolves target", func(t *testing.T) {
		files := parseKotlinFiles(t,
			"Entities.kt", `package db

annotation class Entity(val indices: Array<Index> = [])
annotation class Index(vararg val value: String)
annotation class PrimaryKey

@Entity(indices = [Index("userId")])
class Post(@PrimaryKey val id: Long, val userId: Long)
`,
			"Joins.kt", `package db

annotation class Embedded
annotation class Relation(val parentColumn: String, val entityColumn: String, val entity: kotlin.reflect.KClass<*> = Any::class)

class UserWithPosts(
    @Relation(parentColumn = "id", entityColumn = "userId", entity = Post::class)
    val postIds: List<Long>,
)
`,
		)
		findings := runCrossFileRule(rule, files)
		if len(findings) != 0 {
			t.Fatalf("expected no findings when entity is indexed, got %d: %v", len(findings), findings)
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

func TestEntityMutableColumn_Positive(t *testing.T) {
	findings := runRuleByName(t, "EntityMutableColumn", `
package test

annotation class Entity
annotation class PrimaryKey

@Entity
data class User(
    @PrimaryKey val id: Long,
    var name: String,
)`)
	if len(findings) == 0 {
		t.Fatal("expected finding for @Entity var column")
	}
}

func TestEntityMutableColumn_Negative(t *testing.T) {
	findings := runRuleByName(t, "EntityMutableColumn", `
package test

annotation class Entity
annotation class PrimaryKey

@Entity
data class User(
    @PrimaryKey val id: Long,
    val name: String,
)

data class DraftUser(
    var name: String,
)`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for all-val @Entity or non-Entity var, got %v", findings)
	}
}

func TestSqliteCursorWithoutClose_Positive(t *testing.T) {
	findings := runRuleByName(t, "SqliteCursorWithoutClose", `
package test

interface Cursor {
    fun moveToNext(): Boolean
    fun close()
}

interface SQLiteDatabase {
    fun rawQuery(sql: String, args: Array<String>?): Cursor
}

fun loadUsers(db: SQLiteDatabase) {
    val cursor = db.rawQuery("SELECT * FROM users", null)
    while (cursor.moveToNext()) {
    }
}
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for cursor without use or close")
	}
}

func TestSqliteCursorWithoutClose_NegativeUse(t *testing.T) {
	findings := runRuleByName(t, "SqliteCursorWithoutClose", `
package test

interface Cursor {
    fun moveToNext(): Boolean
    fun close()
}

interface SQLiteDatabase {
    fun rawQuery(sql: String, args: Array<String>?): Cursor
}

inline fun <T : Cursor, R> T.use(block: (T) -> R): R = block(this)

fun loadUsers(db: SQLiteDatabase) {
    db.rawQuery("SELECT * FROM users", null).use { cursor ->
        while (cursor.moveToNext()) {
        }
    }
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when cursor is wrapped in use, got %v", findings)
	}
}

func TestSqliteCursorWithoutClose_NegativeExplicitClose(t *testing.T) {
	findings := runRuleByName(t, "SqliteCursorWithoutClose", `
package test

interface Cursor {
    fun moveToNext(): Boolean
    fun close()
}

interface SQLiteDatabase {
    fun rawQuery(sql: String, args: Array<String>?): Cursor
}

fun loadUsers(db: SQLiteDatabase) {
    val cursor = db.rawQuery("SELECT * FROM users", null)
    while (cursor.moveToNext()) {
    }
    cursor.close()
}
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when cursor.close() is present, got %v", findings)
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

func TestRoomExportSchemaDisabled_Positive(t *testing.T) {
	findings := runRuleByName(t, "RoomExportSchemaDisabled", `
package test

annotation class Database(
    val entities: Array<Any> = [],
    val version: Int = 1,
    val exportSchema: Boolean = true,
)

class RoomDatabase

class User

@Database(entities = [User::class], version = 3, exportSchema = false)
abstract class AppDb : RoomDatabase()
`)
	if len(findings) == 0 {
		t.Fatal("expected finding for @Database(exportSchema = false)")
	}
}

func TestRoomExportSchemaDisabled_NegativeDefault(t *testing.T) {
	findings := runRuleByName(t, "RoomExportSchemaDisabled", `
package test

annotation class Database(
    val entities: Array<Any> = [],
    val version: Int = 1,
    val exportSchema: Boolean = true,
)

class RoomDatabase

class User

@Database(entities = [User::class], version = 3)
abstract class AppDb : RoomDatabase()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when exportSchema is omitted, got %v", findings)
	}
}

func TestRoomExportSchemaDisabled_NegativeTrue(t *testing.T) {
	findings := runRuleByName(t, "RoomExportSchemaDisabled", `
package test

annotation class Database(
    val entities: Array<Any> = [],
    val version: Int = 1,
    val exportSchema: Boolean = true,
)

class RoomDatabase

class User

@Database(entities = [User::class], version = 3, exportSchema = true)
abstract class AppDb : RoomDatabase()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings when exportSchema = true, got %v", findings)
	}
}

func TestRoomExportSchemaDisabled_NegativeNotDatabase(t *testing.T) {
	findings := runRuleByName(t, "RoomExportSchemaDisabled", `
package test

annotation class SomethingElse(val exportSchema: Boolean = true)

class Base

@SomethingElse(exportSchema = false)
class NotARoomDb : Base()
`)
	if len(findings) != 0 {
		t.Fatalf("expected no findings for non-@Database annotation, got %v", findings)
	}
}

