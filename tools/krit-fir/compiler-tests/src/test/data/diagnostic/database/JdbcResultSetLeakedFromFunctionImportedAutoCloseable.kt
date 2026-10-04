// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12
// Positive, like Go: a return type spelled ResultSet that is AutoCloseable
// itself, through an import alias. Go reports it by name; it returns a
// closeable value the caller has to close, so the message holds.
package test

import java.lang.AutoCloseable as ResultSet

<!JdbcResultSetLeakedFromFunction!>fun<!> importAliasSelf(c: ResultSet): ResultSet = c

<!JdbcResultSetLeakedFromFunction!>fun<!> importAliasSelfNullable(c: ResultSet?): ResultSet? = c
