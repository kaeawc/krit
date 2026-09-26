// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: static SQL, schema-constant interpolation, prepared statements,
// no-argument calls, and non-JDBC lookalikes are not reported, as in Go.
package test

import java.sql.CallableStatement
import java.sql.Connection
import java.sql.PreparedStatement
import java.sql.Statement

private const val USERS_TABLE = "users"
private const val COLUMN_ID = "id"

object Schema {
    const val TABLE_ACCOUNTS = "accounts"
}

class SafeDao {
    fun literals(conn: Connection, stmt: Statement) {
        conn.createStatement().executeQuery("SELECT * FROM users")
        stmt.execute("""
            DELETE FROM users
        """.trimIndent())
        stmt.executeUpdate("DELETE FROM users " + "WHERE id = 1")
    }

    // Go treats schema constant names as static data.
    fun schemaConstants(stmt: Statement) {
        stmt.executeQuery("SELECT * FROM $USERS_TABLE WHERE $COLUMN_ID = 1")
        stmt.executeQuery("SELECT * FROM ${Schema.TABLE_ACCOUNTS}")
        stmt.executeQuery("SELECT * FROM " + USERS_TABLE)
        stmt.executeQuery(Schema.TABLE_ACCOUNTS)
    }

    // PreparedStatement and CallableStatement are skipped: their String
    // overloads throw rather than run the SQL.
    fun prepared(conn: Connection, prepared: PreparedStatement, callable: CallableStatement, id: String) {
        val ps = conn.prepareStatement("SELECT * FROM users WHERE id = ?")
        ps.executeQuery()
        prepared.executeQuery("SELECT * FROM users WHERE id = $id")
        callable.execute("CALL purge(" + id + ")")
        conn.prepareStatement("SELECT 1").execute("SELECT * FROM users WHERE id = $id")
    }

    // Unqualified calls have no receiver to type, in Go or here.
    fun implicitReceiver(stmt: Statement, id: String) {
        with(stmt) {
            executeQuery("SELECT * FROM users WHERE id = $id")
        }
    }

    // Other JDBC methods are out of scope.
    fun otherMethods(stmt: Statement, id: String) {
        stmt.addBatch("DELETE FROM users WHERE id = $id")
    }
}

// A lookalike class with the same method names is not a JDBC Statement.
class FakeStatement {
    fun execute(sql: String): Boolean = sql.isEmpty()
    fun executeQuery(sql: String): String = sql
}

fun lookalike(fake: FakeStatement, id: String) {
    fake.execute("DELETE FROM users WHERE id = $id")
    fake.executeQuery("SELECT " + id)
}
