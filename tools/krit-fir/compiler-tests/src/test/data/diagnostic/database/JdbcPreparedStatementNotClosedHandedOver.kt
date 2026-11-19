// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 56, 61, 67, 73
// A statement handed to other code (returned, passed on, or stored) is closed
// by its new owner, so this scope does not leak it. A property holding a value
// read from the statement does not hand the statement over, and neither does
// a call on it whose result is dropped.
package test

import java.sql.Connection
import java.sql.PreparedStatement
import java.sql.ResultSet

fun closeQuietly(resource: AutoCloseable?) {
    resource?.close()
}

fun returned(conn: Connection, id: Int): PreparedStatement {
    val stmt = conn.prepareStatement("SELECT * FROM t WHERE id = ?")
    stmt.setInt(1, id)
    return stmt
}

fun passedOn(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    try {
        stmt.execute()
    } finally {
        closeQuietly(stmt)
    }
}

class Batch(private val conn: Connection) : AutoCloseable {
    private val open = mutableListOf<PreparedStatement>()

    fun add(sql: String) {
        val stmt = conn.prepareStatement(sql)
        open += stmt
    }

    override fun close() = open.forEach { it.close() }
}

// Returned through calls that return their receiver.
fun returnedThroughApply(conn: Connection): PreparedStatement {
    val stmt = conn.prepareStatement("SELECT 1")
    return stmt.apply { setInt(1, 1) }
}

fun returnedThroughTakeIf(conn: Connection?): PreparedStatement? {
    val stmt = conn?.prepareStatement("SELECT 1")
    return stmt?.takeIf { it.maxRows > 0 }
}

// The property holds a value read from the statement: the statement stays open.
fun countReturned(conn: Connection): Int {
    <!JdbcPreparedStatementNotClosed!>val<!> updated = conn.prepareStatement("UPDATE t SET x = 1").executeUpdate()
    return updated
}

fun resultSetReturned(conn: Connection): ResultSet {
    <!JdbcPreparedStatementNotClosed!>val<!> rs = conn.prepareStatement("SELECT 1").executeQuery()
    return rs
}

// A call whose result is dropped, or read again, hands nothing over.
fun applyAsStatement(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
    stmt.apply { setInt(1, 1) }
    stmt.execute()
}

fun applyThenRead(conn: Connection): Int {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
    return stmt.apply { setInt(1, 1) }.executeUpdate()
}
