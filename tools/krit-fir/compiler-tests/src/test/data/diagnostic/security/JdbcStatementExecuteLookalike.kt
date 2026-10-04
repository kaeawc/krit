// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A project class named Statement in another package is not
// java.sql.Statement, even with the same method names.
package test.fakejdbc

interface Statement {
    fun execute(sql: String): Boolean
    fun executeQuery(sql: String): String
}

fun lookalike(stmt: Statement, id: String) {
    stmt.execute("DELETE FROM users WHERE id = $id")
    stmt.executeQuery("SELECT * FROM users WHERE id = " + id)
}
