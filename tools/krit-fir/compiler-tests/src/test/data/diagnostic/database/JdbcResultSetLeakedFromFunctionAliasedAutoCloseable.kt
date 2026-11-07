// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positive: a return type spelled ResultSet that is AutoCloseable itself,
// through a type alias. Go needs java.sql evidence in the file and finds none
// here, so it misses it; it returns a closeable value the caller has to close,
// so the message holds. AutoCloseable has only Any above it, so the checker
// must accept the class itself, not only its supertypes.
package test

typealias ResultSet = AutoCloseable

<!JdbcResultSetLeakedFromFunction!>fun<!> aliasSelf(c: AutoCloseable): ResultSet = c

<!JdbcResultSetLeakedFromFunction!>fun<!> aliasSelfNullable(c: AutoCloseable?): ResultSet? = c

<!JdbcResultSetLeakedFromFunction!>fun<!> aliasSelfQualified(c: AutoCloseable): test.ResultSet = c
