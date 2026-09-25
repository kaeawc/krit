// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence: functions that return a java.sql.ResultSet the Go rule misses,
// because Go reads only an explicit return type's text and needs it to end in
// `ResultSet`. Each returns a JDBC ResultSet the caller has to close, so each
// is reported here.
package test

import java.sql.ResultSet as Rows
import java.sql.Statement

typealias Cursor = java.sql.ResultSet

// Go misses it: no declared return type; the inferred type is ResultSet.
<!JdbcResultSetLeakedFromFunction!>fun<!> inferred(stmt: Statement) = stmt.executeQuery("SELECT 1")

// Go misses it: the declared type is a type alias of ResultSet.
<!JdbcResultSetLeakedFromFunction!>fun<!> typeAliased(stmt: Statement): Cursor = stmt.executeQuery("SELECT 1")

// Go misses it: the declared type is an import alias of ResultSet.
<!JdbcResultSetLeakedFromFunction!>fun<!> importAliased(stmt: Statement): Rows = stmt.executeQuery("SELECT 1")

// Go misses it: Go reads the last dotted segment of `(java.sql.ResultSet)`, which is
// `ResultSet)`, not `ResultSet`.
<!JdbcResultSetLeakedFromFunction!>fun<!> parenthesized(stmt: Statement): (java.sql.ResultSet) = stmt.executeQuery("SELECT 1")
