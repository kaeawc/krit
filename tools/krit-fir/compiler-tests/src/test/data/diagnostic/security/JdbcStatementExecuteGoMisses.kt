// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 32, 48, 51
// Go misses: FIR reads the receiver's resolved type, so it reports these
// java.sql.Statement calls, most of which Go cannot type (comments say which).
// Go types only a bare receiver name (a Statement parameter or local, or a
// local initialized by `conn.createStatement()` in the same function) or
// `conn.createStatement()` on a bare name resolved to Connection.
package test

import java.sql.Connection
import java.sql.DriverManager
import java.sql.Statement
import javax.sql.DataSource

class StatementHolder(val stmt: Statement)

abstract class LoggingStatement : Statement

fun openStatement(conn: Connection): Statement = conn.createStatement()

class GoMissesDao(private val stmt: Statement) {
    // The `use` lambda's `it` is the Statement.
    fun useBlock(conn: Connection, id: String) {
        conn.createStatement().use { it.executeQuery(<!JdbcStatementExecute!>"SELECT * FROM users WHERE id = $id"<!>) }
        conn.createStatement().use { s -> s.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = " + id<!>) }
    }

    // A property chain receiver. Go reports `this.stmt` (it reads the
    // receiver as `stmt` and resolves the class property) but not
    // `holder.stmt`.
    fun propertyChains(holder: StatementHolder, id: String) {
        this.stmt.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
        holder.stmt.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
    }

    // A connection from a call or a property chain.
    fun callChains(url: String, dataSource: DataSource, id: String) {
        DriverManager.getConnection(url).createStatement().executeQuery(<!JdbcStatementExecute!>"SELECT * FROM users WHERE id = $id"<!>)
        dataSource.connection.createStatement().executeQuery(<!JdbcStatementExecute!>"SELECT * FROM users WHERE id = $id"<!>)
    }

    // A statement from a project function, a cast, or a smart cast. Go
    // reports the bare local and the smart-cast name, whose types its resolver
    // infers, but not the call chain or the cast.
    fun otherSources(conn: Connection, any: Any, id: String) {
        openStatement(conn).execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
        val fromHelper = openStatement(conn)
        fromHelper.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
        (any as Statement).execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
        if (any is Statement) {
            any.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
        }
    }

    // A Statement subtype.
    fun subtype(logging: LoggingStatement, id: String) {
        logging.executeQuery(<!JdbcStatementExecute!>"SELECT * FROM users WHERE id = $id"<!>)
    }

    // A statement created outside a function body: Go looks for the
    // receiver's declaration only inside an enclosing function.
    val initialized: Boolean = DriverManager.getConnection("jdbc:h2:mem:").createStatement().let { s ->
        s.execute(<!JdbcStatementExecute!>"CREATE TABLE " + System.getenv("TABLE")<!>)
    }
}

// A not-null assertion, a parenthesized receiver, and a receiver typed by a
// type parameter bounded by Statement: Go reads only a bare receiver name
// and cannot type the type parameter.
fun <T : Statement> assertedAndGeneric(s: Statement?, p: Statement, t: T, id: String) {
    s!!.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
    (p).execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
    t.execute(<!JdbcStatementExecute!>"DELETE FROM users WHERE id = $id"<!>)
}
