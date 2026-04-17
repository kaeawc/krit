package test

import java.sql.ResultSet

interface Statement {
    fun executeQuery(sql: String): ResultSet
}

fun query(stmt: Statement, sql: String): ResultSet =
    stmt.executeQuery(sql)
