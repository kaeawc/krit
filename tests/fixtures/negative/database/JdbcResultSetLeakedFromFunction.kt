package test

import java.sql.ResultSet

interface Statement {
    fun executeQuery(sql: String): ResultSet
}

inline fun <R> ResultSet.use(block: (ResultSet) -> R): R = block(this)

fun <R> query(stmt: Statement, sql: String, block: (ResultSet) -> R): R =
    stmt.executeQuery(sql).use(block)
