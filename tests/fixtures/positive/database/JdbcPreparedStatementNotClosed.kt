package test

interface Connection {
    fun prepareStatement(sql: String): PreparedStatement
}

interface PreparedStatement {
    fun executeQuery(): ResultSet
    fun close()
}

interface ResultSet

fun query(connection: Connection) {
    val stmt = connection.prepareStatement("SELECT 1")
    val rs = stmt.executeQuery()
    println(rs)
}

// Regression: a leaked short-named statement `s` must FIRE even when a
// longer-named sibling `vs.close()` is present in the same scope. The
// substring scan that this rule used to use was matching `vs.close(`
// as evidence that `s` had been closed.
fun queryWithLookalike(connection: Connection) {
    val s = connection.prepareStatement("SELECT 1")
    val vs = connection.prepareStatement("SELECT 2")
    val rs = s.executeQuery()
    try {
        vs.close()
    } finally {
        // s is never closed
        println(rs)
    }
}
