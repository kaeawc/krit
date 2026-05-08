package test

import java.sql.Connection

private const val USERS_TABLE = "users"
private const val COLUMN_ID = "id"

class JdbcStatementExecuteSafeFixture {
    fun load(connection: Connection) {
        connection.createStatement().executeQuery("SELECT * FROM users")
        connection.createStatement().executeQuery("SELECT * FROM $USERS_TABLE WHERE $COLUMN_ID = 1")
        val prepared = connection.prepareStatement("SELECT * FROM users WHERE id = ?")
        prepared.executeQuery()
    }
}
