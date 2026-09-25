// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 16, 18, 20, 24, 26, 29
// Divergence: Go reports every function below; none returns a closeable
// ResultSet, so the message ("returns ResultSet; callers almost always forget
// to close it") is not true of them. No finding here.
package test

import java.sql.Statement

// Go reports these because the last dotted segment of the function type's
// text is `ResultSet`. Each returns a function, not a ResultSet; the caller
// gets a ResultSet only by invoking it. The unqualified `() -> ResultSet`
// (JdbcResultSetLeakedFromFunctionNegative.kt) is left alone by Go too.
fun deferredQualified(stmt: Statement): () -> java.sql.ResultSet = { stmt.executeQuery("SELECT 1") }

fun receiverFunction(): Statement.() -> java.sql.ResultSet = { executeQuery("SELECT 1") }

fun suspendFunction(stmt: Statement): suspend () -> java.sql.ResultSet = { stmt.executeQuery("SELECT 1") }

fun parameterFunction(): (Statement) -> java.sql.ResultSet = { it.executeQuery("SELECT 1") }

// Go reports these because the type parameter is named ResultSet. Its bounds
// are not closeable (Any? or CharSequence), so there is nothing to close.
fun <ResultSet> typeParamPlain(value: ResultSet): ResultSet = value

fun <ResultSet : CharSequence> typeParamNotCloseable(value: ResultSet): ResultSet = value

class Holder<ResultSet>(private val value: ResultSet) {
    fun get(): ResultSet = value
}
