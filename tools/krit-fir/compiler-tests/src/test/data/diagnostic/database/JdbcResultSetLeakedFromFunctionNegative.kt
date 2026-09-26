// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives the Go rule also leaves alone: functions without a body, a
// ResultSet consumed through a block, a ResultSet only nested in the return
// type, a type parameter bounded by ResultSet, and ResultSets returned from
// lambdas, anonymous functions, and property getters, none of which is a
// function declaration.
package test

import java.sql.ResultSet
import java.sql.Statement

interface Source {
    fun abstractQuery(): ResultSet
}

abstract class Base {
    abstract fun abstractQuery(): ResultSet
}

fun <R> query(stmt: Statement, sql: String, block: (ResultSet) -> R): R =
    stmt.executeQuery(sql).use(block)

fun count(stmt: Statement): Int = stmt.executeQuery("SELECT COUNT(*) FROM t").use { rs ->
    rs.next()
    rs.getInt(1)
}

fun wrapped(stmt: Statement): List<ResultSet> = listOf(stmt.executeQuery("SELECT 1"))

fun deferred(stmt: Statement): () -> ResultSet = { stmt.executeQuery("SELECT 1") }

fun paired(stmt: Statement): Pair<ResultSet, Int> = stmt.executeQuery("SELECT 1") to 1

fun <T : ResultSet> bounded(value: T): T = value

fun unit(stmt: Statement) {
    stmt.executeQuery("SELECT 1").close()
}

fun anonymous(stmt: Statement): Any = fun(): ResultSet = stmt.executeQuery("SELECT 1")

class Holder(private val stmt: Statement) {
    val current: ResultSet
        get() = stmt.executeQuery("SELECT 1")

    val explicitGetter: ResultSet
        get(): ResultSet {
            return stmt.executeQuery("SELECT 1")
        }
}

// The generated componentN() and copy() have no source of their own.
data class Row(val rs: ResultSet)

// Delegated members are generated; Go sees no function declaration either.
class Delegating(source: Source) : Source by source

fun lambdas(stmt: Statement): Int {
    val open: () -> ResultSet = { stmt.executeQuery("SELECT 1") }
    return open().row
}
