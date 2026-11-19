// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14
// Go reports this: it reads only the class's direct supertypes, and the
// override's java.sql member comes through the project's own base class. The
// JDBC API fixes the return type, so no finding here.
package test

import java.sql.ResultSet
import java.sql.Statement

abstract class BaseStatement(protected val delegate: Statement) : Statement by delegate

abstract class LoggingStatement(delegate: Statement) : BaseStatement(delegate) {
    override fun executeQuery(sql: String?): ResultSet = delegate.executeQuery(sql)
}
