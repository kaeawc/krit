// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 40, 41, 42
// Project functions named like the JDBC methods. One that takes SQL text as
// its first parameter and runs on a Statement (a thin extension wrapper) is
// reported like the JDBC call, as in Go: the runtime-built SQL still runs on
// a java.sql.Statement. One whose first parameter is not SQL text (a query
// object) or whose receiver is not a plain Statement is not.
package test

import java.sql.Connection
import java.sql.PreparedStatement
import java.sql.ResultSet
import java.sql.Statement

fun Statement.executeQuery(sql: String, timeoutSeconds: Int): ResultSet {
    queryTimeout = timeoutSeconds
    return executeQuery(sql)
}

fun Statement.execute(sql: String, logIt: Boolean): Boolean {
    if (logIt) println(sql)
    return execute(sql)
}

fun <T : Statement> T.executeUpdate(sql: String?, tag: String): Int {
    println(tag)
    return executeUpdate(sql)
}

fun Connection.execute(sql: String): Boolean = prepareStatement(sql).use { it.execute() }

class TypedQuery(val sql: String)

interface QueryStatement : Statement {
    fun executeQuery(query: TypedQuery): ResultSet
}

class WrapperDao {
    fun wrappers(stmt: Statement, id: String) {
        stmt.executeQuery(<!JdbcStatementExecute!>"SELECT * FROM users WHERE id = " + id<!>, 5)
        stmt.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>, true)
        stmt.executeUpdate(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = " + id<!>, "purge")
    }

    // A member overload on a Statement subtype that takes a query object.
    fun queryObject(q: QueryStatement, query: TypedQuery) {
        q.executeQuery(query)
    }

    // A wrapper called on a PreparedStatement, and an extension on a
    // Connection, do not run the SQL on a plain Statement.
    fun notStatement(ps: PreparedStatement, conn: Connection, id: String) {
        ps.executeQuery("SELECT * FROM users WHERE id = " + id, 5)
        conn.execute("DELETE FROM users WHERE id = $id")
    }
}
