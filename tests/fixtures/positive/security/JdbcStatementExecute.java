package test;

import java.sql.Connection;
import java.sql.Statement;

class JdbcStatementExecuteJavaFixture {
    void load(Connection connection, String id) throws Exception {
        Statement stmt = connection.createStatement();
        stmt.executeQuery("SELECT * FROM users WHERE id = " + id);
    }
}
