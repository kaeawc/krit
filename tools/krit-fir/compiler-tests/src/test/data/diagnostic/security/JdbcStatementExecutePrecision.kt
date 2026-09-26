// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 21, 22, 24, 26, 28, 35, 38, 39, 40, 51, 59, 84, 85, 86, 87, 88, 93, 94, 98, 99, 100, 118, 119, 120
// Go artifacts FIR drops: each Go finding here asserts something the code
// does not have.
package test

import java.sql.Connection
import java.sql.Statement

private const val SELECT_ALL = "SELECT * FROM users"
private const val QUERY = "SELECT * FROM users"
private const val WHERE = " WHERE id = 1"

class PrecisionDao {
    // Go reports "built with non-static concatenation" on these, reading any
    // name other than a schema-constant name as runtime data. The SQL is a
    // compile-time constant, a local val holding one, or a `+` of those, so
    // nothing in it is non-static.
    fun constants(stmt: Statement) {
        stmt.executeQuery(QUERY)
        stmt.executeQuery(QUERY + WHERE)
        stmt.executeQuery("SELECT * FROM users" + WHERE)
        val sql = "SELECT * FROM users"
        stmt.executeQuery(sql)
        val filtered = sql + " WHERE id = 1"
        stmt.executeQuery(filtered)
        // A literal with an escaped dollar: Go reads the `$` as data.
        stmt.executeQuery("SELECT * FROM prices WHERE currency = '\$'")
    }

    // Still reported: a local var, a local val holding runtime data, and a
    // non-const top-level property are not proven static.
    fun notConstant(stmt: Statement, id: String) {
        var sql = "SELECT * FROM users"
        stmt.executeQuery(<!JdbcStatementExecute!>sql<!>)
        sql = "SELECT * FROM users WHERE id = $id"
        val computed = "SELECT * FROM users WHERE id = " + id
        stmt.executeQuery(<!JdbcStatementExecute!>computed<!>)
        stmt.executeQuery(<!JdbcStatementExecute!>defaultQuery<!>)
        stmt.executeQuery(<!JdbcStatementExecute!>SELECT_ALL + id<!>)
    }

    // Go binds `stmt` to the earlier `createStatement()` declaration of the
    // same name in another scope; this `stmt` is not a JDBC Statement.
    fun shadowedLookalike(conn: Connection, id: String) {
        run {
            val stmt = conn.createStatement()
            stmt.close()
        }
        val stmt = SqlRunner()
        stmt.execute("DELETE FROM users WHERE id = $id")
    }

    // A project extension named like the JDBC method whose first parameter
    // is a query object, not SQL text (Go reports the argument as computed
    // SQL); the extension's own inner call is where the SQL runs. A wrapper
    // that takes SQL text is reported (JdbcStatementExecuteWrappers.kt).
    fun extension(stmt: Statement, query: UserQuery) {
        stmt.executeQuery(query)
    }
}

var defaultQuery = "SELECT * FROM users"

class SqlRunner {
    fun execute(sql: String): Boolean = sql.isEmpty()
}

class UserQuery(val sql: String)

fun Statement.executeQuery(query: UserQuery): Boolean = execute(query.sql)

private const val pageSize = 10

// Go reports "built with non-static concatenation" on these too: it reads a
// non-String literal operand (an Int, Long, Boolean, or Char), a lower-case
// const, a name holding an if or when over literals, and a final val
// initialized with a literal as runtime data. Every value here is fixed in
// the source, so nothing in the SQL is non-static.
class StaticValueDao(private val admin: Boolean) {
    private val selectAll = "SELECT * FROM users"

    fun literals(stmt: Statement) {
        stmt.executeQuery("SELECT * FROM users LIMIT " + 10)
        stmt.executeQuery("SELECT * FROM users LIMIT " + 10L)
        stmt.executeQuery("SELECT * FROM users WHERE active = " + true)
        stmt.executeQuery("SELECT * FROM users WHERE grade = " + 'A')
        stmt.executeQuery("SELECT * FROM users LIMIT " + pageSize)
    }

    fun branches(stmt: Statement) {
        val sql = if (admin) "SELECT * FROM users" else "SELECT id FROM users"
        stmt.executeQuery(sql)
        stmt.executeQuery(when { admin -> "SELECT * FROM users"; else -> "SELECT id FROM users" })
    }

    fun finalVals(stmt: Statement) {
        stmt.executeQuery(selectAll)
        stmt.executeQuery(fromCompanion)
        stmt.executeQuery(topLevelQuery)
    }

    companion object {
        val fromCompanion = "SELECT * FROM users"
    }
}

val topLevelQuery = "SELECT * FROM users"

// Still reported: a branch holding runtime data, an open val (an override
// may compute it), and a val with a custom getter are not proven static.
open class NotStaticDao(private val admin: Boolean, private val id: String) {
    open val overridable = "SELECT * FROM users"
    val computedGetter: String get() = "SELECT * FROM users WHERE id = " + id

    fun notStatic(stmt: Statement) {
        val sql = if (admin) "SELECT * FROM users" else "SELECT * FROM users WHERE id = " + id
        stmt.executeQuery(<!JdbcStatementExecute!>sql<!>)
        stmt.executeQuery(<!JdbcStatementExecute!>overridable<!>)
        stmt.executeQuery(<!JdbcStatementExecute!>computedGetter<!>)
    }
}
