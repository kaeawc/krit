// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 23, 25, 34, 59, 61, 73
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

// Like Go: a class named ResultSet that declares close() without implementing
// AutoCloseable. The caller still has to close it, so the message's claim that
// callers forget to close it holds; only `.use {}` does not apply. This is a
// judgment call, so it matches Go.
class Cursor {
    class ResultSet {
        fun close() {}
    }
}

<!JdbcResultSetLeakedFromFunction!>fun<!> closeByMethod(): Cursor.ResultSet = Cursor.ResultSet()

<!JdbcResultSetLeakedFromFunction!>fun<!> closeByMethodNullable(): Cursor.ResultSet? = null

open class Releasable {
    fun close(force: Boolean) {}
}

class Pooled {
    class ResultSet : Releasable()
}

// Like Go: close() is inherited and takes an argument; it still has to be
// called.
<!JdbcResultSetLeakedFromFunction!>fun<!> inheritedClose(): Pooled.ResultSet = Pooled.ResultSet()

// Divergence: Go misses it. No declared return type; the inferred type is the
// closeable Driver.ResultSet.
<!JdbcResultSetLeakedFromFunction!>fun<!> inferredCloseable() = Driver.ResultSet()

// Divergence: Go misses it, as above; the inferred type is Cursor.ResultSet,
// which declares close().
<!JdbcResultSetLeakedFromFunction!>fun<!> inferredCloseByMethod() = Cursor.ResultSet()

typealias DriverRows = Driver.ResultSet

// Divergence: Go misses it. Go reads the declared type's text, `DriverRows`;
// the alias expands to the closeable Driver.ResultSet.
<!JdbcResultSetLeakedFromFunction!>fun<!> aliasedCloseable(): DriverRows = Driver.ResultSet()
