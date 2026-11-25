package com.example

import com.google.errorprone.annotations.CompileTimeConstant

object Database {
    fun execSql(@CompileTimeConstant sql: String) {}
}

class UserDao {
    fun deleteUser(table: String) {
        Database.execSql("DELETE FROM $table")
    }

    fun deleteAll() {
        var sql = "DELETE FROM users"
        Database.execSql(sql)
    }
}

class Settings {
    @CompileTimeConstant
    val query = "SELECT * FROM settings"
}
