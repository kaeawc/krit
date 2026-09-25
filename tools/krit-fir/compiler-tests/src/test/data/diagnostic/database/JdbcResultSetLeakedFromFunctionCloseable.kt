// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive, like Go: a return type named ResultSet that is not
// java.sql.ResultSet but is closeable (a wrapper, another driver's cursor).
// Go reports it by name; the message holds, since the caller must close it and
// `.use {}` applies, so it is reported here too.
package test

import java.io.Closeable

class ResultSet(private val delegate: java.sql.ResultSet) : Closeable {
    override fun close() = delegate.close()
}

class Driver {
    class ResultSet : AutoCloseable {
        override fun close() {}
    }
}

<!JdbcResultSetLeakedFromFunction!>fun<!> wrap(delegate: java.sql.ResultSet): ResultSet = ResultSet(delegate)

<!JdbcResultSetLeakedFromFunction!>fun<!> wrapNullable(delegate: java.sql.ResultSet?): ResultSet? = delegate?.let(::ResultSet)

<!JdbcResultSetLeakedFromFunction!>fun<!> cursor(): Driver.ResultSet = Driver.ResultSet()

// A local class: its lookup tag is bound to the local symbol, so nothing is
// resolved by class id.
fun localClass(): Int {
    class ResultSet : AutoCloseable {
        override fun close() {}
    }

    <!JdbcResultSetLeakedFromFunction!>fun<!> open(): ResultSet = ResultSet()
    return open().hashCode()
}

class Pool {
    class ResultSet<T>(val value: T) : AutoCloseable {
        override fun close() {}
    }
}

// Divergence: Go misses it. Go reads the last dotted segment of the declared
// type's text, `ResultSet<Int>`, which is not `ResultSet`. It returns a
// closeable ResultSet all the same.
<!JdbcResultSetLeakedFromFunction!>fun<!> pooled(): Pool.ResultSet<Int> = Pool.ResultSet(1)
