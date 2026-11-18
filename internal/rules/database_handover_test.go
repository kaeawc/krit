package rules_test

import "testing"

func assertLines(t *testing.T, rule string, got []int, want ...int) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: got lines %v, want %v", rule, got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("%s: got lines %v, want %v", rule, got, want)
		}
	}
}

func TestJdbcPreparedStatementNotClosed_HandedOver(t *testing.T) {
	findings := runRuleByName(t, "JdbcPreparedStatementNotClosed", `package test

import java.sql.Connection
import java.sql.PreparedStatement

fun closeQuietly(resource: AutoCloseable?) { resource?.close() }

fun returned(conn: Connection): PreparedStatement {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt.setInt(1, 1)
    return stmt
}

fun passedOn(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    closeQuietly(stmt)
}

fun returnedThroughApply(conn: Connection): PreparedStatement {
    val stmt = conn.prepareStatement("SELECT 1")
    return stmt.apply { setInt(1, 1) }.also { it.maxRows = 1 }
}

fun derivedReturned(conn: Connection): Int {
    val n = conn.prepareStatement("UPDATE t SET x = 1").executeUpdate()
    return n
}

fun applyAsStatement(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt.apply { setInt(1, 1) }
    stmt.execute()
}

fun invoked(conn: Connection) {
    val make = { conn.prepareStatement("SELECT 1") }
    make().execute()
}

fun checkedForNull(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    if (stmt != null) stmt.execute()
}

fun typeChecked(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    if (stmt is AutoCloseable) stmt.execute()
}

fun notNullAsserted(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt!!.execute()
}
`)
	var got []int
	for _, f := range findings {
		got = append(got, f.Line)
	}
	assertLines(t, "JdbcPreparedStatementNotClosed", got, 25, 30, 36, 41, 46, 51)
}

func TestSqliteCursorWithoutClose_DerivedValueHandedOn(t *testing.T) {
	findings := runRuleByNameWithResolver(t, "SqliteCursorWithoutClose", `package test

import android.database.Cursor
import android.database.sqlite.SQLiteDatabase

fun countReturned(db: SQLiteDatabase): Int {
    val n = db.rawQuery("SELECT 1", null).count
    return n
}

fun booleanPassedOn(db: SQLiteDatabase) {
    val any = db.rawQuery("SELECT 1", null).moveToFirst()
    println(any)
}

fun returned(db: SQLiteDatabase): Cursor {
    val cursor = db.rawQuery("SELECT 1", null)
    return cursor
}

fun returnedThroughApply(db: SQLiteDatabase): Cursor {
    val cursor = db.rawQuery("SELECT 1", null)
    return cursor.apply { moveToFirst() }
}

fun elvis(db: SQLiteDatabase?): Cursor? {
    val cursor = db?.rawQuery("SELECT 1", null) ?: return null
    return cursor
}
`)
	var got []int
	for _, f := range findings {
		got = append(got, f.Line)
	}
	assertLines(t, "SqliteCursorWithoutClose", got, 7, 12)
}

func TestJdbcResultSetLeakedFromFunction_JdbcOverrides(t *testing.T) {
	findings := runRuleByName(t, "JdbcResultSetLeakedFromFunction", `package test

import java.sql.DatabaseMetaData
import java.sql.ResultSet
import java.sql.Statement

abstract class DelegatingStatement(private val delegate: Statement) : Statement by delegate {
    override fun executeQuery(sql: String?): ResultSet = delegate.executeQuery(sql)
}

abstract class TracingMetaData(private val delegate: DatabaseMetaData) : DatabaseMetaData {
    override fun getCatalogs(): ResultSet = delegate.catalogs
}

fun wrap(delegate: Statement): Statement = object : Statement by delegate {
    override fun getGeneratedKeys(): ResultSet = delegate.generatedKeys
}

interface Source {
    fun rows(): ResultSet
}

class Impl(private val stmt: Statement) : Source {
    override fun rows(): ResultSet = stmt.executeQuery("SELECT 1")
}
`)
	var got []int
	for _, f := range findings {
		got = append(got, f.Line)
	}
	assertLines(t, "JdbcResultSetLeakedFromFunction", got, 24)
}

func TestJdbcResultSetLeakedFromFunction_OtherImportedResultSet(t *testing.T) {
	findings := runRuleByName(t, "JdbcResultSetLeakedFromFunction", `package test

import com.example.cassandra.ResultSet
import java.sql.Connection

fun rows(session: Session): ResultSet = session.execute("SELECT 1")

fun jdbc(conn: Connection): java.sql.ResultSet = conn.createStatement().executeQuery("SELECT 1")
`)
	var got []int
	for _, f := range findings {
		got = append(got, f.Line)
	}
	assertLines(t, "JdbcResultSetLeakedFromFunction", got, 8)
}
