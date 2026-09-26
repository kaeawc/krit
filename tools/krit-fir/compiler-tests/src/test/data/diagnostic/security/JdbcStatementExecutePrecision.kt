// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 21, 22, 24, 26, 28, 35, 38, 39, 40, 51, 57
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

    // A project extension named like the JDBC method takes a query object,
    // not SQL text.
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
