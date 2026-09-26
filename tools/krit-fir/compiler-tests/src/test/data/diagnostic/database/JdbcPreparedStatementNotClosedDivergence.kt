// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 18, 23, 28, 35, 36, 37, 44, 54, 55, 68
// Divergences from Go, each decided by the message ("PreparedStatement '<name>'
// should be wrapped in use { } or explicitly closed with .close().").
package test

import java.sql.Connection

// Go reports these: its cleanup search wants the exact text `<name>.close(`
// or `<name>.use`, so a safe call, a `!!` receiver, a backticked receiver, or
// a parenthesized receiver is missed. Each statement is closed, so no finding.
fun safeCallClose(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt?.close()
}

fun notNullUse(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt!!.use { it.execute() }
}

fun backtickedClose(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    `stmt`.close()
}

fun parenthesizedClose(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    (stmt).close()
}

// Go reports these: any prepareStatement call inside the initializer counts,
// even one closed on the spot. The statement is closed, so no finding.
fun closedOnTheSpot(conn: Connection) {
    val count = conn.prepareStatement("UPDATE t SET x = 1").use { it.executeUpdate() }
    val done = conn.prepareStatement("SELECT 1")?.close()
    val rows = conn.prepareStatement("SELECT 1")!!.use { it.executeQuery().next() }
    println(listOf(count, done, rows))
}

// Go reports `result` too: the statement is made by the nested `stmt`, which
// is closed. `result` holds no statement, so no finding.
fun nestedProperty(conn: Connection) {
    val result = run {
        val stmt = conn.prepareStatement("SELECT 1")
        stmt.use { it.executeQuery().next() }
    }
    println(result)
}

// Go reports both `result` and the nested `stmt` when the nested one leaks.
// Only `stmt` holds the statement; `result` is a Boolean.
fun nestedLeak(conn: Connection) {
    val result = run {
        <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
        stmt.execute()
    }
    println(result)
}

// Go reports this: the close is in a member declared above the property, and
// Go reads only the declarations after it. The statement is closed.
class ClosedAbove(conn: Connection) : AutoCloseable {
    override fun close() {
        statement.close()
    }

    private val statement = conn.prepareStatement("SELECT 1")
}

// Go leaves these alone: the text `stmt.close(` / `stmt.use` appears in a
// comment, a string, or a call that is not `use`. Nothing closes the
// statement, so FIR reports it.
fun closeInComment(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
    // TODO stmt.close()
    stmt.execute()
}

fun closeInString(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
    println("remember stmt.close() later")
    stmt.execute()
}

fun useLookalike(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
    stmt.useless()
}

fun java.sql.PreparedStatement.useless() {}

// Go leaves this alone: tree-sitter has no property declaration for a `when`
// subject. The subject is a property holding a statement nobody closes.
fun whenSubject(conn: Connection) {
    when (<!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")) {
        else -> stmt.execute()
    }
}
