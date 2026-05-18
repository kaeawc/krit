package test

interface Connection {
    fun prepareStatement(sql: String): PreparedStatement
}

interface PreparedStatement {
    fun executeQuery(): ResultSet
    fun close()
}

interface ResultSet

inline fun <T : PreparedStatement, R> T.use(block: (T) -> R): R = block(this)

fun query(connection: Connection) {
    val stmt = connection.prepareStatement("SELECT 1")
    stmt.executeQuery()
    stmt.close()
}

fun querySafely(connection: Connection) {
    connection.prepareStatement("SELECT 1").use { stmt ->
        stmt.executeQuery()
    }
}

// Regression: a short-named statement `s` is properly closed; the
// receiver-bound identifier-boundary check must recognise `s.close()`
// (and must not be confused into thinking some other identifier ending
// in 's' closed it).
fun queryShortName(connection: Connection) {
    val s = connection.prepareStatement("SELECT 1")
    try {
        s.executeQuery()
    } finally {
        s.close()
    }
}
