// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 28, 30, 47, 48, 49
// Lookalikes: a `prepareStatement` of another library. Go matches the call by
// name only.
package test

// A closeable statement of another library is a statement to close: reported
// as Go reports it.
class Statement : AutoCloseable {
    fun run() {}

    override fun close() {}
}

class Session {
    fun prepareStatement(sql: String): Statement = Statement()
}

class Legacy {
    fun close() {}
}

class LegacySession {
    fun prepareStatement(sql: String): Legacy = Legacy()
}

fun closeable(session: Session, legacy: LegacySession) {
    <!JdbcPreparedStatementNotClosed!>val<!> stmt = session.prepareStatement("SELECT 1")
    stmt.run()
    <!JdbcPreparedStatementNotClosed!>val<!> old = legacy.prepareStatement("SELECT 1")
    println(old)
}

// Divergence: Go reports these because the call is named prepareStatement.
// None returns anything closeable, so there is no statement to close.
class Query(val sql: String)

class Builder {
    fun prepareStatement(sql: String): Query = Query(sql)
}

fun prepareStatement(sql: String): String = sql.trim()

fun <T> prepareStatement(value: T, sql: String): T = value

fun notCloseable(builder: Builder) {
    val query = builder.prepareStatement("SELECT 1")
    val text = prepareStatement("SELECT 1")
    val generic = prepareStatement(1, "SELECT 1")
    println(listOf(query.sql, text, generic))
}
