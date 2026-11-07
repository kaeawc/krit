package rules_test

import (
	"testing"

	api "github.com/kaeawc/krit/internal/rules/api"
	"github.com/kaeawc/krit/internal/scanner"
	"github.com/kaeawc/krit/internal/typeinfer"
)

var crossFileAuditRules = map[string]bool{
	"RoomConflictStrategyReplaceOnFk": true,
	"RoomRelationWithoutIndex":        true,
	"RoomFlowQueryWithoutDistinct":    true,
	"DatabaseQueryOnMainThread":       true,
}

type auditCase struct {
	name string
	rule string
	code string
	want int
}

func runAuditCases(t *testing.T, cases []auditCase) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var got []scanner.Finding
			if crossFileAuditRules[c.rule] {
				files := parseKotlinFiles(t, "A.kt", c.code)
				resolver := typeinfer.NewResolver()
				resolver.IndexFilesParallel(files, 1)
				ctx := &api.Context{
					Resolver:    resolver,
					CodeIndex:   scanner.BuildIndex(files, 1),
					ParsedFiles: files,
					Collector:   scanner.NewFindingCollector(0),
				}
				buildRuleIndex()[c.rule].Check(ctx)
				cols := *ctx.Collector.Columns()
				for i := 0; i < cols.Len(); i++ {
					got = append(got, cols.Finding(i))
				}
			} else {
				got = findingsFor(t, buildRuleIndex()[c.rule], parseInline(t, c.code))
			}
			if len(got) != c.want {
				t.Errorf("%s: want %d findings, got %d: %v", c.rule, c.want, len(got), got)
			}
		})
	}
}

func TestAuditDatabaseAnnotationBoundaries(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"dao lookalike annotation", "DaoNotInterface", `
package test
annotation class DaoHelper
@DaoHelper
abstract class Foo
`, 0},
		{"qualified dao", "DaoNotInterface", `
package test
@androidx.room.Dao
abstract class Foo
`, 1},
		{"database view not database", "RoomExportSchemaDisabled", `
package test
import androidx.room.DatabaseView
@DatabaseView("SELECT 1", viewName = "v")
class V(val exportSchema: Boolean = false)
`, 0},
		{"dao upsert and rawquery", "DaoWithoutAnnotations", `
package test
import androidx.room.*
@Dao
interface UserDao {
    @Upsert fun up(u: User)
    @RawQuery fun raw(q: SupportSQLiteQuery): List<User>
}
class User
`, 0},
		{"abstract dao concrete helper", "DaoWithoutAnnotations", `
package test
import androidx.room.*
@Dao
abstract class UserDao {
    @Insert abstract fun ins(u: User)
    open fun helper(u: User) { ins(u) }
}
class User
`, 0},
		{"unannotated abstract", "DaoWithoutAnnotations", `
package test
import androidx.room.*
@Dao
interface UserDao {
    fun nothing(u: User)
}
class User
`, 1},
		{"fk positional", "ForeignKeyWithoutOnDelete", `
package test
import androidx.room.*
@Entity(foreignKeys = [ForeignKey(User::class, ["id"], ["uid"], ForeignKey.CASCADE)])
class Post(val uid: Long)
class User
`, 0},
		{"fk missing", "ForeignKeyWithoutOnDelete", `
package test
import androidx.room.*
@Entity(foreignKeys = [ForeignKey(entity = User::class, parentColumns = ["id"], childColumns = ["uid"])])
class Post(val uid: Long)
class User
`, 1},
		{"resultset domain class", "JdbcResultSetLeakedFromFunction", `
package test
class ResultSet(val rows: List<String>)
fun load(): ResultSet { return ResultSet(emptyList()) }
`, 0},
		{"resultset jdbc", "JdbcResultSetLeakedFromFunction", `
package test
import java.sql.ResultSet
fun load(): ResultSet { TODO() }
`, 1},
		{"stmt safe call close", "JdbcPreparedStatementNotClosed", `
package test
import java.sql.Connection
fun f(c: Connection) {
    val stmt = c.prepareStatement("select 1")
    stmt?.executeQuery()
    stmt?.close()
}
`, 0},
		{"stmt not closed", "JdbcPreparedStatementNotClosed", `
package test
import java.sql.Connection
fun f(c: Connection) {
    val stmt = c.prepareStatement("select 1")
    stmt.executeQuery()
}
`, 1},
		{"stmt in use lambda arg", "JdbcPreparedStatementNotClosed", `
package test
import java.sql.Connection
fun f(c: Connection) {
    c.prepareStatement("select 1").use { it.executeQuery() }
}
`, 0},
		{"stmt prepared inside use of connection", "JdbcPreparedStatementNotClosed", `
package test
import java.sql.Connection
fun f(c: Connection) {
    val stmt = c.prepareStatement("select 1")
    return stmt.use { it.executeQuery() }
}
`, 0},
		{"stmt returned still reported like FIR", "JdbcPreparedStatementNotClosed", `
package test
import java.sql.Connection
fun f(c: Connection): java.sql.PreparedStatement {
    val stmt = c.prepareStatement("select 1")
    return stmt
}
`, 1},
	})
}

func TestAuditDatabaseRoomShapes(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"singleton holder", "DatabaseInstanceRecreated", `
package test
import androidx.room.Room
import android.content.Context
abstract class AppDb {
    companion object {
        @Volatile private var INSTANCE: AppDb? = null
        fun get(context: Context): AppDb =
            INSTANCE ?: synchronized(this) {
                val i = Room.databaseBuilder(context, AppDb::class.java, "a").build()
                INSTANCE = i
                i
            }
    }
}
`, 0},
		{"lazy holder", "DatabaseInstanceRecreated", `
package test
import androidx.room.Room
import android.content.Context
abstract class AppDb
class Holder(private val context: Context) {
    val db: AppDb by lazy { Room.databaseBuilder(context, AppDb::class.java, "a").build() }
}
`, 0},
		{"application onCreate", "DatabaseInstanceRecreated", `
package test
import androidx.room.Room
import android.content.Context
abstract class AppDb
class App {
    lateinit var db: AppDb
    fun onCreate(context: Context) {
        db = Room.databaseBuilder(context, AppDb::class.java, "a").build()
    }
}
`, 0},
		{"koin single", "DatabaseInstanceRecreated", `
package test
import androidx.room.Room
import android.content.Context
abstract class AppDb
fun single(block: () -> Any) {}
val dbModule = run {
    single { Room.databaseBuilder(Context(), AppDb::class.java, "a").build() }
}
class Context
`, 0},
		{"replace list insert fk", "RoomConflictStrategyReplaceOnFk", `
package test
import androidx.room.*
@Entity(foreignKeys = [ForeignKey(entity = User::class, parentColumns = ["id"], childColumns = ["uid"], onDelete = ForeignKey.CASCADE)])
class Post(val uid: Long)
class User
@Dao
interface PostDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertAll(posts: List<Post>)
}
`, 1},
		{"replace vararg insert fk", "RoomConflictStrategyReplaceOnFk", `
package test
import androidx.room.*
@Entity(foreignKeys = [ForeignKey(entity = User::class, parentColumns = ["id"], childColumns = ["uid"], onDelete = ForeignKey.CASCADE)])
class Post(val uid: Long)
class User
@Dao
interface PostDao {
    @Insert(onConflict = OnConflictStrategy.REPLACE)
    suspend fun insertAll(vararg posts: Post)
}
`, 1},
		{"relation column info index", "RoomRelationWithoutIndex", `
package test
import androidx.room.*
@Entity
class Post(@PrimaryKey val id: Long, @ColumnInfo(index = true) val userId: Long)
class UserWithPosts(
    @Embedded val user: User,
    @Relation(parentColumn = "id", entityColumn = "userId") val posts: List<Post>
)
class User
`, 0},
		{"relation body property index", "RoomRelationWithoutIndex", `
package test
import androidx.room.*
@Entity
class Post(@PrimaryKey val id: Long) {
    @ColumnInfo(index = true) var userId: Long = 0
}
class UserWithPosts(
    @Embedded val user: User,
    @Relation(parentColumn = "id", entityColumn = "userId") val posts: List<Post>
)
class User
`, 0},
		{"relation really missing", "RoomRelationWithoutIndex", `
package test
import androidx.room.*
@Entity
class Post(@PrimaryKey val id: Long, val userId: Long)
class UserWithPosts(
    @Embedded val user: User,
    @Relation(parentColumn = "id", entityColumn = "userId") val posts: List<Post>
)
class User
`, 1},
		{"multiple writes other receiver", "RoomMultipleWritesMissingTransaction", `
package test
import androidx.room.*
@Dao
abstract class D {
    @Insert abstract fun insert(x: Int)
    fun f(other: Other) { other.insert(1); other.insert(2) }
}
class Other { fun insert(x: Int) {} }
`, 0},
		{"multiple writes missing", "RoomMultipleWritesMissingTransaction", `
package test
import androidx.room.*
@Dao
abstract class D {
    @Insert abstract fun insert(x: Int)
    @Delete abstract fun delete(x: Int)
    open fun replace(x: Int) { delete(x); insert(x) }
}
`, 1},
		{"suspend transaction atomic writes", "RoomSuspendQueryInTransaction", `
package test
import androidx.room.*
@Dao
abstract class D {
    @Query("DELETE FROM t") abstract suspend fun deleteAll()
    @Insert abstract suspend fun insertAll(x: List<Int>)
    @Transaction open suspend fun replaceAll(x: List<Int>) { deleteAll(); insertAll(x) }
}
`, 0},
		{"select star single entity", "RoomSelectStarWithoutLimit", `
package test
import androidx.room.*
@Dao
interface D {
    @Query("SELECT * FROM t WHERE id = :id") suspend fun byId(id: Long): T?
}
class T
`, 0},
		{"select star list", "RoomSelectStarWithoutLimit", `
package test
import androidx.room.*
@Dao
interface D {
    @Query("SELECT * FROM t") suspend fun all(): List<T>
}
class T
`, 1},
		{"select star rx single list", "RoomSelectStarWithoutLimit", `
package test
import androidx.room.*
@Dao
interface D {
    @Query("SELECT * FROM t") fun all(): Single<List<T>>
}
class T
class Single<T>
`, 1},
		{"query lookalike", "RoomQueryMissingWhereForUpdate", `
package test
annotation class QueryHelper(val v: String)
interface D {
    @QueryHelper("DELETE FROM t") fun x()
}
`, 0},
		{"delete no where", "RoomQueryMissingWhereForUpdate", `
package test
import androidx.room.*
@Dao
interface D {
    @Query("DELETE FROM t") fun x()
}
`, 1},
	})
}

func TestAuditRoomFlowQueryWithoutDistinct(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"first is one shot", "RoomFlowQueryWithoutDistinct", `
package test
import androidx.room.*
import kotlinx.coroutines.flow.*
@Dao
interface D { @Query("SELECT 1") fun observe(): Flow<Int> }
suspend fun f(d: D) { d.observe().first() }
`, 0},
		{"collected", "RoomFlowQueryWithoutDistinct", `
package test
import androidx.room.*
import kotlinx.coroutines.flow.*
@Dao
interface D { @Query("SELECT 1") fun observe(): Flow<Int> }
fun f(d: D) = d.observe().map { it + 1 }
`, 1},
		{"name shared with list dao", "RoomFlowQueryWithoutDistinct", `
package test
import androidx.room.*
import kotlinx.coroutines.flow.*
@Dao
interface D { @Query("SELECT 1") fun all(): Flow<Int> }
@Dao
interface E { @Query("SELECT 1") fun all(): List<Int> }
fun f(e: E) = e.all().map { it + 1 }
`, 0},
	})
}

func TestAuditRoomMigrationAndBuilderShapes(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"anonymous migration object interpolation", "RoomMigrationUsesExecSqlWithInterpolation", `
package test
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
val M12 = object : Migration(1, 2) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE t ADD COLUMN c TEXT DEFAULT '${"$"}{name}'")
    }
}
val name = "x"
`, 1},
		{"constant interpolation", "RoomMigrationUsesExecSqlWithInterpolation", `
package test
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
const val TABLE_NAME = "t"
object M23 : Migration(2, 3) {
    override fun migrate(db: SupportSQLiteDatabase) {
        db.execSQL("ALTER TABLE $TABLE_NAME ADD COLUMN c TEXT")
        db.execSQL("ALTER TABLE ${Tables.USERS} ADD COLUMN c TEXT")
    }
}
object Tables { const val USERS = "u" }
`, 0},
		{"variable interpolation", "RoomMigrationUsesExecSqlWithInterpolation", `
package test
import androidx.room.migration.Migration
import androidx.sqlite.db.SupportSQLiteDatabase
object M23 : Migration(2, 3) {
    override fun migrate(db: SupportSQLiteDatabase) {
        val col = "c"
        db.execSQL("ALTER TABLE t ADD COLUMN $col TEXT")
    }
}
`, 1},
		{"negated debug guard still release", "RoomFallbackToDestructiveMigration", `
package test
import androidx.room.RoomDatabase
fun f(b: RoomDatabase.Builder<*>) {
    if (!BuildConfig.DEBUG) { b.fallbackToDestructiveMigration() }
}
object BuildConfig { const val DEBUG = true }
`, 1},
		{"else of debug guard is release", "RoomFallbackToDestructiveMigration", `
package test
import androidx.room.RoomDatabase
fun f(b: RoomDatabase.Builder<*>) {
    if (BuildConfig.DEBUG) { println() } else { b.fallbackToDestructiveMigration() }
}
object BuildConfig { const val DEBUG = true }
`, 1},
		{"debug guard", "RoomFallbackToDestructiveMigration", `
package test
import androidx.room.RoomDatabase
fun f(b: RoomDatabase.Builder<*>) {
    if (BuildConfig.DEBUG) { b.fallbackToDestructiveMigration() }
}
object BuildConfig { const val DEBUG = true }
`, 0},
	})
}

func TestAuditDatabaseMisc(t *testing.T) {
	runAuditCases(t, []auditCase{
		{"pk var autogenerate", "EntityPrimaryKeyNotStable", `
package test
import androidx.room.*
@Entity
data class U(@PrimaryKey(autoGenerate = true) var id: Long = 0, val name: String)
`, 0},
		{"pk var no autogenerate", "EntityPrimaryKeyNotStable", `
package test
import androidx.room.*
@Entity
data class U(@PrimaryKey var id: Long, val name: String)
`, 1},
		{"pk var in non entity", "EntityPrimaryKeyNotStable", `
package test
import androidx.room.*
data class U(@PrimaryKey var id: Long)
`, 0},
		{"cursor use via question", "SqliteCursorWithoutClose", `
package test
import android.database.sqlite.SQLiteDatabase
fun f(db: SQLiteDatabase) {
    val c = db.rawQuery("select 1", null)
    c?.close()
}
`, 0},
		{"cursor closed in finally", "SqliteCursorWithoutClose", `
package test
import android.database.sqlite.SQLiteDatabase
fun f(db: SQLiteDatabase) {
    val c = db.rawQuery("select 1", null)
    try { c.moveToFirst() } finally { c.close() }
}
`, 0},
		{"cursor leaked", "SqliteCursorWithoutClose", `
package test
import android.database.sqlite.SQLiteDatabase
fun f(db: SQLiteDatabase) {
    val c = db.rawQuery("select 1", null)
    c.moveToFirst()
}
`, 1},
		{"cursor returned", "SqliteCursorWithoutClose", `
package test
import android.database.sqlite.SQLiteDatabase
import android.database.Cursor
fun f(db: SQLiteDatabase): Cursor {
    val c = db.rawQuery("select 1", null)
    return c
}
`, 0},
		{"like escape embedding", "RoomQueryWithLikeMissingEscape", `
package test
import androidx.room.*
@Dao
interface D { @Query("SELECT * FROM t WHERE name LIKE '%' || :q || '%'") fun s(q: String): List<String> }
`, 0},
		{"dao abstract class", "DaoNotInterface", `
package test
import androidx.room.*
@Dao
abstract class D { @Query("SELECT 1") abstract fun s(): Int }
`, 1},
		{"log guard missing", "LogLevelGuardMissing", `
package test
import org.slf4j.LoggerFactory
private val logger = LoggerFactory.getLogger("x")
fun expensive(): String = ""
fun f() { logger.debug("value ${expensive()}") }
`, 1},
		{"log guard present", "LogLevelGuardMissing", `
package test
import org.slf4j.LoggerFactory
private val logger = LoggerFactory.getLogger("x")
fun expensive(): String = ""
fun f() { if (logger.isDebugEnabled) { logger.debug("value ${expensive()}") } }
`, 0},
	})
}
