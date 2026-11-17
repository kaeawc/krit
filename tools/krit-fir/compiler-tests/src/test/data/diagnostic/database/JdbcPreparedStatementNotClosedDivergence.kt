// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 28, 35, 36, 37, 42, 43, 62, 75, 112, 118, 124, 135, 144
// Divergences from Go, each decided by the message ("PreparedStatement '<name>'
// should be wrapped in use { } or explicitly closed with .close().").
package test

import java.sql.Connection

// Go reports the backticked and parenthesized receivers: its cleanup search
// wants the text `<name>.close(`, `<name>?.close(`, `<name>!!.close(` or
// `<name>.use`. Each statement is closed, so no finding.
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

fun nullableClosedOnTheSpot(conn: Connection?) {
    val done = conn?.prepareStatement("SELECT 1")?.close()
    val used = conn?.prepareStatement("SELECT 1")?.use { it.execute() }
    println(listOf(done, used))
}

// Go leaves `result` alone because it reads `println(result)` as a hand-over
// of whatever `run` returns. The statement is made by the nested `stmt`, which
// is closed, and `result` holds no statement, so no finding either way.
fun nestedProperty(conn: Connection) {
    val result = run {
        val stmt = conn.prepareStatement("SELECT 1")
        stmt.use { it.executeQuery().next() }
    }
    println(result)
}

// Both report the nested `stmt`, which leaks. Go leaves `result` alone because
// it reads `println(result)` as a hand-over; `result` is a Boolean anyway.
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

// Go reports these: the receiver and the cleanup call are split by a line
// break or a comment, so the text `stmt.close(` / `stmt.use` never appears.
// Each statement is closed, so no finding.
fun multilineClose(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt
        .close()
}

fun multilineUse(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt
        .use { it.execute() }
}

fun commentedClose(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt /* done */ .close()
}

// Go reports this: the close is in a top-level function declared above the
// property, and Go reads only the declarations after it. The statement is
// closed.
fun closedAboveTop() {
    topStmt.close()
}

val topStmt = divergenceConnection().prepareStatement("SELECT 1")

fun divergenceConnection(): Connection = TODO()

// Both report the member `s` of the anonymous object, which makes the
// statement. Go leaves `holder` alone because it reads `println(holder)` as a
// hand-over; `holder` is not a statement, so no finding on it.
fun anonymousHolder(conn: Connection) {
    val holder = object {
        <!JdbcPreparedStatementNotClosed!>val<!> s = conn.prepareStatement("SELECT 1")
    }
    println(holder)
}

// Go leaves these alone (a setter on its own line, or after `;`, is not part
// of tree-sitter's property declaration), and so does FIR: the setter's
// statement is not the property's value, which is an Int.
class Setters(private val conn: Connection) {
    var counter: Int = 0
        set(v) {
            conn.prepareStatement("SELECT 1").execute()
            field = v
        }

    var counter2: Int = 0; set(v) { conn.prepareStatement("SELECT 1").execute(); field = v }
}
