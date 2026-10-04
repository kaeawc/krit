// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 14, 16, 18, 20, 23, 32
// Positive, like Go: a return type that is a type parameter named ResultSet
// whose bounds make it closeable. Go reports it by name; the returned value is
// closeable and the caller has to close it, so the message holds.
package test

import java.io.Closeable

<!JdbcResultSetLeakedFromFunction!>fun<!> <ResultSet : AutoCloseable> typeParam(value: ResultSet): ResultSet = value

<!JdbcResultSetLeakedFromFunction!>fun<!> <ResultSet : AutoCloseable> typeParamNullable(value: ResultSet?): ResultSet? = value

<!JdbcResultSetLeakedFromFunction!>fun<!> <ResultSet : Closeable> closeableBound(value: ResultSet): ResultSet = value

<!JdbcResultSetLeakedFromFunction!>fun<!> <ResultSet : java.sql.ResultSet> jdbcBound(value: ResultSet): ResultSet = value

<!JdbcResultSetLeakedFromFunction!>fun<!> <ResultSet> whereBound(value: ResultSet): ResultSet where ResultSet : CharSequence, ResultSet : AutoCloseable = value

<!JdbcResultSetLeakedFromFunction!>fun<!> <C : AutoCloseable, ResultSet : C> chainedBound(value: ResultSet): ResultSet = value

class Box<ResultSet : AutoCloseable>(private val value: ResultSet) {
    <!JdbcResultSetLeakedFromFunction!>fun<!> get(): ResultSet = value
}

class Handle {
    fun close() {}
}

// The bound declares close() without implementing AutoCloseable, as for a
// class named ResultSet (see JdbcResultSetLeakedFromFunctionCloseable.kt).
<!JdbcResultSetLeakedFromFunction!>fun<!> <ResultSet : Handle> closeMethodBound(value: ResultSet): ResultSet = value

// Divergence: Go misses it. No declared return type; the inferred type is the
// closeable type parameter ResultSet.
<!JdbcResultSetLeakedFromFunction!>fun<!> <ResultSet : AutoCloseable> inferredTypeParam(value: ResultSet) = value
