// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 16, 21
// A comparison, a type check or a `!!` only inspects the statement; none of
// them hands it to code that closes it, so the statement still leaks.
package test

import java.sql.Connection
import java.sql.PreparedStatement

fun checkedForNull(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt: PreparedStatement? = conn.prepareStatement("SELECT 1")
    if (stmt != null) stmt.execute()
}

fun typeChecked(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt: Any = conn.prepareStatement("SELECT 1")
    if (stmt is PreparedStatement) stmt.execute()
}

fun notNullAsserted(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt: PreparedStatement? = conn.prepareStatement("SELECT 1")
    stmt!!.execute()
}
