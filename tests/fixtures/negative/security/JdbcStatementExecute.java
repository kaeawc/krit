package test;

import java.sql.Connection;
import java.sql.PreparedStatement;

class JdbcStatementExecuteJavaSafeFixture {
    void load(Connection connection) throws Exception {
        connection.createStatement().executeQuery("SELECT * FROM users");
        PreparedStatement stmt = connection.prepareStatement("SELECT * FROM users WHERE id = ?");
        stmt.executeQuery();
    }
}
