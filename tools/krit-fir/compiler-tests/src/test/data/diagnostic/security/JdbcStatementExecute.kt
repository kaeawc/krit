// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 18, 22, 23, 24, 25, 29, 33, 37, 43, 50, 55
// Positive: a java.sql.Statement execute* call whose SQL argument is
// interpolated or computed from non-static data, reported on the argument
// with the message for its shape. Every shape here is one Go also reports.
package test

import java.sql.Connection
import java.sql.Statement

class UserDao(private val connection: Connection) {
    fun chained(conn: Connection, id: String) {
        conn.createStatement().executeQuery(<!JdbcStatementExecute!>"SELECT * FROM users WHERE id = $id"<!>)
    }

    fun localStatement(conn: Connection, id: String) {
        val stmt = conn.createStatement()
        stmt.executeUpdate(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = " + id<!>)
    }

    fun statementParameter(stmt: Statement, id: String, sql: String) {
        stmt.execute(<!JdbcStatementExecute!>"UPDATE users SET seen = 1 WHERE id = ${id.trim()}"<!>)
        stmt.execute(<!JdbcStatementExecute!>sql<!>)
        stmt.executeLargeUpdate(<!JdbcStatementExecute!>sql<!>)
        stmt.executeUpdate(<!JdbcStatementExecute!>sql<!>, Statement.RETURN_GENERATED_KEYS)
    }

    fun safeCall(stmt: Statement?, id: String) {
        stmt?.executeQuery(<!JdbcStatementExecute!>"SELECT * FROM users WHERE id = '$id'"<!>)
    }

    fun parenthesized(stmt: Statement, id: String) {
        stmt.execute(<!JdbcStatementExecute!>("DELETE FROM users WHERE id = " + id)<!>)
    }

    fun computedCall(stmt: Statement, filter: StringBuilder) {
        stmt.executeQuery(<!JdbcStatementExecute!>filter.toString()<!>)
    }

    // Go reports on the argument's first line.
    fun multiLine(stmt: Statement, id: String) {
        stmt.executeQuery(
            <!JdbcStatementExecute!>"SELECT * FROM users WHERE id = " + id<!>,
        )
    }

    // An interpolated name that is not a schema constant name still counts,
    // even when a schema constant sits beside it.
    fun mixedInterpolation(stmt: Statement, name: String) {
        stmt.executeQuery(<!JdbcStatementExecute!>"SELECT * FROM $USERS_TABLE WHERE name = '$name'"<!>)
    }

    // A class property typed Connection.
    fun propertyConnection(id: String) {
        connection.createStatement().execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
    }
}

private const val USERS_TABLE = "users"
