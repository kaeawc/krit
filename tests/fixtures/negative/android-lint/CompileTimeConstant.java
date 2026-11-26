package com.example;

import com.google.errorprone.annotations.CompileTimeConstant;

public class Database {
    static final String USERS = "users";
    @CompileTimeConstant static final String DEFAULT_QUERY = "SELECT 1";

    static void execSql(@CompileTimeConstant String sql) {}

    static void forward(@CompileTimeConstant final String sql) {
        execSql(sql);
    }

    void deleteAll() {
        execSql("DELETE FROM " + USERS);
        final String vacuum = "VACUUM";
        execSql(vacuum);
        execSql(DEFAULT_QUERY);
    }
}
