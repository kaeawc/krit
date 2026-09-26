// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 22, 29, 37, 48, 55, 60, 65, 70, 76, 83, 89, 91, 93, 105, 112, 118, 121, 126
// Positive: a property that makes a JDBC PreparedStatement with no later
// `<name>.close()` or `<name>.use { }` in its scope. Each is reported on the
// property's first line (its first annotation or modifier, else `val`/`var`),
// the line the Go rule reports.
package test

import java.sql.Connection
import java.sql.PreparedStatement

class Closer {
    fun close() {}
}

fun local(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
    stmt.executeQuery()
}

fun variable(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>var<!> stmt: PreparedStatement? = conn.prepareStatement("SELECT 1")
    stmt?.executeQuery()
    stmt = null
}

// Receivers whose names only end with the statement's name close something else.
fun lookalikeReceivers(conn: Connection, apsCloser: Closer, mystmt: Closer) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
    stmt.executeQuery()
    apsCloser.close()
    mystmt.close()
}

// Closing a sibling statement does not close this one.
fun sibling(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> s = conn.prepareStatement("SELECT 1")
    val vs = conn.prepareStatement("SELECT 2")
    s.executeQuery()
    vs.close()
}

// A close before the declaration (of an outer same-named value) is not a later cleanup.
fun closedBefore(conn: Connection, other: PreparedStatement) {
    other.close()
    run {
        other.close()
        <!JdbcPreparedStatementNotClosed!>val<!> other = conn.prepareStatement("SELECT 1")
        other.executeQuery()
    }
}

// The result set leaks the statement it came from.
fun chained(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> rs = conn.prepareStatement("SELECT 1").executeQuery()
    rs.next()
}

fun safeCall(conn: Connection?) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn?.prepareStatement("SELECT 1")
    stmt?.executeQuery()
}

fun annotated(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>@Suppress("UNUSED_VARIABLE")<!>
    val stmt = conn.prepareStatement("SELECT 1")
}

fun backticked(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> `the stmt` = conn.prepareStatement("SELECT 1")
    `the stmt`.executeQuery()
}

// A lambda that makes the statement: every call leaks one.
fun deferred(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> make = { conn.prepareStatement("SELECT 1") }
    make().executeQuery()
}

// Statements made in a lambda body are checked in that scope.
fun inLambda(conn: Connection) {
    listOf("SELECT 1").forEach { sql ->
        <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement(sql)
        stmt.execute()
    }
}

class Repository(private val conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>private<!> val insert = conn.prepareStatement("INSERT INTO t VALUES (?)")

    <!JdbcPreparedStatementNotClosed!>val<!> lazyStatement by lazy { conn.prepareStatement("SELECT 1") }

    <!JdbcPreparedStatementNotClosed!>val<!> fresh: PreparedStatement get() = conn.prepareStatement("SELECT 1")

    // Go misses this one: with the getter on its own line, tree-sitter does
    // not nest it in the property declaration, so Go sees no
    // prepareStatement call in the property. Every read makes a new statement
    // that nobody closes, as for `fresh`.
    <!JdbcPreparedStatementNotClosed!>val<!> freshMultiline: PreparedStatement
        get() = conn.prepareStatement("SELECT 1")

    /**
     * KDoc is not part of the reported line.
     */
    <!JdbcPreparedStatementNotClosed!>internal<!> val documented = conn.prepareStatement("SELECT 1")

    fun run() {
        insert.execute()
    }

    companion object {
        <!JdbcPreparedStatementNotClosed!>val<!> shared = openConnection().prepareStatement("SELECT 1")
    }
}

fun openConnection(): Connection = TODO()

<!JdbcPreparedStatementNotClosed!>val<!> topLevel = openConnection().prepareStatement("SELECT 1")

fun anonymousObject(conn: Connection): Any = object {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
}

fun localClass(conn: Connection) {
    class Local {
        <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.prepareStatement("SELECT 1")
    }
    Local()
}
