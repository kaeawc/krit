// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19
// Receiver shapes. A type alias of Statement is reported, as in Go. Go misses
// a member of an object expression or of a local class that implements
// Statement (it reads only a bare name typed exactly java.sql.Statement);
// both run the SQL on a Statement, so they are reported. The checker takes
// the owner from its lookup tag and compares class ids without resolving
// them, so the anonymous and local owners do not throw. An object expression
// that is not a Statement is not reported.
package test

import java.sql.Connection
import java.sql.Statement

typealias Stmt = Statement

class ReceiverDao {
    fun aliased(s: Stmt, id: String) {
        s.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
    }

    fun objectExpression(conn: Connection, id: String) {
        val wrapped = object : Statement by conn.createStatement() {
            override fun toString(): String = "wrapped"
        }
        wrapped.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
        val runner = object {
            fun execute(sql: String): Boolean = sql.isEmpty()
        }
        runner.execute("DELETE FROM users WHERE id = $id")
    }

    fun localClass(any: Any, id: String) {
        abstract class LocalStmt : Statement
        val local = any as? LocalStmt
        local?.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
    }
}
