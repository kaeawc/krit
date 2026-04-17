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
