// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// An override of a java.sql member keeps the return type the JDBC API fixes,
// so there is no (ResultSet) -> R signature to switch to. No finding here; an
// override of the project's own interface is still reported
// (JdbcResultSetLeakedFromFunction.kt).
package test

import java.sql.DatabaseMetaData
import java.sql.ResultSet
import java.sql.Statement

abstract class DelegatingStatement(private val delegate: Statement) : Statement by delegate {
    override fun executeQuery(sql: String?): ResultSet = delegate.executeQuery(sql)

    override fun getResultSet(): ResultSet = delegate.resultSet
}

abstract class TracingMetaData(private val delegate: DatabaseMetaData) : DatabaseMetaData {
    override fun getCatalogs(): ResultSet = delegate.catalogs
}
