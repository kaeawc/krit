// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives the Go rule also leaves alone: a statement closed or used later in
// its scope (however deeply nested the close is), a statement consumed
// without a property, properties that make no statement, and names Go never
// reads (destructuring, parameters).
package test

import java.sql.Connection
import java.sql.PreparedStatement

fun closed(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt.executeQuery()
    stmt.close()
}

fun used(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt.use { it.executeQuery() }
}

fun usedWithFunction(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    stmt.use(PreparedStatement::execute)
}

fun finallyClose(conn: Connection) {
    val s = conn.prepareStatement("SELECT 1")
    try {
        s.executeQuery()
    } finally {
        s.close()
    }
}

fun nestedClose(conn: Connection, flag: Boolean) {
    val stmt = conn.prepareStatement("SELECT 1")
    if (flag) {
        listOf(1).forEach { _ -> stmt.close() }
    }
}

fun closedInArgument(conn: Connection) {
    val stmt = conn.prepareStatement("SELECT 1")
    println(stmt.close())
}

fun inline(conn: Connection) {
    conn.prepareStatement("SELECT 1").use { stmt ->
        stmt.executeQuery()
    }
}

fun noStatement(conn: Connection) {
    val stmt = conn.createStatement()
    val sql = "SELECT 1"
    stmt.close()
    println(sql)
}

class Repository(conn: Connection) {
    private val insert = conn.prepareStatement("INSERT INTO t VALUES (?)")

    fun close() {
        this.insert.close()
    }
}

class Cached(conn: Connection) : AutoCloseable {
    private val query = conn.prepareStatement("SELECT 1")

    override fun close() = query.close()
}

// Only parameters and destructured names hold statements here.
fun parameters(stmt: PreparedStatement, conn: Connection) {
    val (a, b) = conn.prepareStatement("SELECT 1") to 1
    stmt.execute()
    println(a)
    println(b)
}

// The same closeable type with a user-defined `use`.
interface Handle {
    fun close()
}

interface Pool {
    fun prepareStatement(sql: String): Handle
}

inline fun <T : Handle, R> T.use(block: (T) -> R): R = block(this)

fun pooled(pool: Pool) {
    val handle = pool.prepareStatement("SELECT 1")
    handle.use { }
}
