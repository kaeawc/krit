// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positive: a return type spelled ResultSet that is AutoCloseable itself,
// through an import alias. Go needs java.sql evidence in the file and finds
// none here, so it misses it; it returns a closeable value the caller has to
// close, so the message holds.
package test

import java.lang.AutoCloseable as ResultSet

<!JdbcResultSetLeakedFromFunction!>fun<!> importAliasSelf(c: ResultSet): ResultSet = c

<!JdbcResultSetLeakedFromFunction!>fun<!> importAliasSelfNullable(c: ResultSet?): ResultSet? = c
