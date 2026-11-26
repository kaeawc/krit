package com.example

import com.google.errorprone.annotations.CompileTimeConstant

const val USERS_TABLE = "users"

object Database {
    fun execSql(@CompileTimeConstant sql: String) {}

    fun execAll(@CompileTimeConstant statements: List<String>) {}

    fun forward(@CompileTimeConstant sql: String) {
        execSql(sql)
    }
}

class UserDao {
    @CompileTimeConstant
    private val deleteSql = "DELETE FROM users"

    fun deleteAll(cascade: Boolean) {
        Database.execSql("DELETE FROM users")
        Database.execSql("DELETE FROM $USERS_TABLE")
        Database.execSql(if (cascade) "DELETE CASCADE" else "DELETE")
        val local = "VACUUM"
        Database.execSql(local)
        Database.execAll(listOf("BEGIN", "COMMIT"))
    }
}

class Unannotated {
    fun execSql(sql: String) {}

    fun run(table: String) {
        execSql("DELETE FROM $table")
    }
}
