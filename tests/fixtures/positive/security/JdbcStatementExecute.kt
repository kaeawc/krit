package test

import java.sql.Connection

class JdbcStatementExecuteFixture {
    fun load(connection: Connection, id: String) {
        connection.createStatement().executeQuery("SELECT * FROM users WHERE id = $id")
        val stmt = connection.createStatement()
        stmt.executeUpdate("DELETE FROM users WHERE id = " + id)
    }
}
