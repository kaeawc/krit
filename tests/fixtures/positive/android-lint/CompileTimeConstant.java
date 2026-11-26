package com.example;

import com.google.errorprone.annotations.CompileTimeConstant;

public class Database {
    @CompileTimeConstant public static String defaultQuery = "SELECT 1";

    static void execSql(@CompileTimeConstant String sql) {}

    void deleteUser(String table) {
        execSql("DELETE FROM " + table);
    }
}
