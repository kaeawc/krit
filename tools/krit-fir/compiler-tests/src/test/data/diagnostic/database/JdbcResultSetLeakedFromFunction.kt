// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 16, 21, 23, 25, 27, 32, 35, 38, 40, 43, 47, 53, 59, 63, 67
// Positive: a function with a body whose declared return type is
// java.sql.ResultSet. Each is reported on the function's first line (its
// modifier list, else `fun`), the line the Go rule reports.
package test

import java.sql.Connection
import java.sql.ResultSet
import java.sql.SQLException
import java.sql.Statement

<!JdbcResultSetLeakedFromFunction!>fun<!> query(stmt: Statement, sql: String): ResultSet =
    stmt.executeQuery(sql)

<!JdbcResultSetLeakedFromFunction!>fun<!> blockBody(stmt: Statement, sql: String): ResultSet {
    val rs = stmt.executeQuery(sql)
    return rs
}

<!JdbcResultSetLeakedFromFunction!>fun<!> nullable(stmt: Statement?, sql: String): ResultSet? = stmt?.executeQuery(sql)

<!JdbcResultSetLeakedFromFunction!>fun<!> qualified(stmt: Statement, sql: String): java.sql.ResultSet = stmt.executeQuery(sql)

<!JdbcResultSetLeakedFromFunction!>fun<!> Connection.selectAll(table: String): ResultSet = createStatement().executeQuery("SELECT * FROM $table")

<!JdbcResultSetLeakedFromFunction!>suspend<!> fun suspending(stmt: Statement): ResultSet = stmt.executeQuery("SELECT 1")

/**
 * KDoc is not part of the reported line.
 */
<!JdbcResultSetLeakedFromFunction!>@Throws(SQLException::class)<!>
fun annotated(stmt: Statement): ResultSet = stmt.executeQuery("SELECT 1")

<!JdbcResultSetLeakedFromFunction!>fun<!> `backticked name`(stmt: Statement): ResultSet = stmt.executeQuery("SELECT 1")

class Repository(private val connection: Connection) {
    <!JdbcResultSetLeakedFromFunction!>fun<!> member(): ResultSet = connection.createStatement().executeQuery("SELECT 1")

    <!JdbcResultSetLeakedFromFunction!>private<!> fun privateMember(): ResultSet = connection.createStatement().executeQuery("SELECT 2")

    companion object {
        <!JdbcResultSetLeakedFromFunction!>fun<!> fromCompanion(stmt: Statement): ResultSet = stmt.executeQuery("SELECT 3")
    }

    fun local(stmt: Statement): Int {
        <!JdbcResultSetLeakedFromFunction!>fun<!> inner(): ResultSet = stmt.executeQuery("SELECT 4")
        return inner().row
    }
}

object Queries {
    <!JdbcResultSetLeakedFromFunction!>fun<!> fromObject(stmt: Statement): ResultSet = stmt.executeQuery("SELECT 5")
}

interface Source {
    fun abstractQuery(): ResultSet

    <!JdbcResultSetLeakedFromFunction!>fun<!> defaultQuery(stmt: Statement): ResultSet = stmt.executeQuery("SELECT 6")
}

class Impl(private val stmt: Statement) : Source {
    <!JdbcResultSetLeakedFromFunction!>override<!> fun abstractQuery(): ResultSet = stmt.executeQuery("SELECT 7")
}

val anonymous = object {
    <!JdbcResultSetLeakedFromFunction!>fun<!> fromAnonymous(stmt: Statement): ResultSet = stmt.executeQuery("SELECT 8")
}
