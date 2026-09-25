// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence: Go misses it. Go reads the declared type's text, `DriverCursor`;
// the import alias names a closeable class called ResultSet, which the caller
// has to close, so it is reported here.
package test

import test.Vendor.ResultSet as DriverCursor

class Vendor {
    class ResultSet : AutoCloseable {
        override fun close() {}
    }
}

<!JdbcResultSetLeakedFromFunction!>fun<!> importAliasedCloseable(): DriverCursor = DriverCursor()
