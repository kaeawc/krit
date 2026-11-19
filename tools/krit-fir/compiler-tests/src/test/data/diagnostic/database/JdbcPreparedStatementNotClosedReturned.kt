// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 24, 32, 43, 52, 54, 82, 84
// Positive: a statement made by a nested local property and returned into a
// closeable outer property (or a lambda returning one). The outer property
// holds the statement and is reported; the nested one hands it over (it is the
// lambda's result or returned), so it is not, as in Go. Also the statements
// made in a nested destructuring declaration and those returned as a captured
// type.
package test

import java.sql.Connection
import java.sql.PreparedStatement

fun returnedFromRun(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = run {
        val s = conn.prepareStatement("SELECT 1")
        s.setInt(1, 2)
        s
    }
    stmt.execute()
}

fun returnedFromLet(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = conn.let { c ->
        val s = c.prepareStatement("SELECT 1")
        s
    }
    stmt.execute()
}

fun returnedFromWith(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt: PreparedStatement = with(conn) {
        val prepared = prepareStatement("SELECT 1")
        prepared.setInt(1, 1)
        prepared
    }
    stmt.executeQuery()
}

// A lambda that returns the nested statement: every call leaks one, as for a
// lambda that makes it directly.
fun returnedFromLambda(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> make = {
        val s = conn.prepareStatement("SELECT 1")
        s
    }
    make().execute()
}

class Getters(private val conn: Connection) {
    // Every read of `fresh` makes a new statement that nobody closes.
    <!JdbcPreparedStatementNotClosed!>val<!> fresh: PreparedStatement get() { val s = conn.prepareStatement("SELECT 1"); return s }

    <!JdbcPreparedStatementNotClosed!>val<!> freshBlock: PreparedStatement get() {
        val s = conn.prepareStatement("SELECT 1")
        return s
    }
}

// A destructuring declaration is not checked on its own, so the statement it
// makes is reported on the outer property, a Boolean that cannot hold the
// statement. Go misses it: it reads `println(result)` as a hand-over of
// whatever `run` returns.
fun destructured(conn: Connection) {
    <!JdbcPreparedStatementNotClosed!>val<!> result = run {
        val (a, _) = conn.prepareStatement("SELECT 1") to 1
        a.execute()
    }
    println(result)
}

// A captured type whose bound is a statement is a statement to close.
interface Factory<T> {
    fun prepareStatement(): T
}

interface BoundedFactory<T : AutoCloseable> {
    fun prepareStatement(): T
}

fun captured(f: Factory<out PreparedStatement>, g: BoundedFactory<*>) {
    <!JdbcPreparedStatementNotClosed!>val<!> s = f.prepareStatement()
    s.execute()
    <!JdbcPreparedStatementNotClosed!>val<!> b = g.prepareStatement()
    b.toString()
}

// Divergence: Go reports this because the call is named prepareStatement. An
// `in` projection gives no bound on what the factory makes (a
// Factory<Any> fits), so nothing shows the value is a statement to close.
fun contravariant(f: Factory<in PreparedStatement>) {
    val v = f.prepareStatement()
    println(v)
}
