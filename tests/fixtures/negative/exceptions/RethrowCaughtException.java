package com.example.exceptions;

import java.io.IOException;
import java.sql.SQLException;

class Handler {
    void handle() {
        try {
            riskyOperation();
        } catch (Exception e) {
            throw new IllegalStateException("Operation failed", e);
        }
    }

    // Intentional: let IOException escape instead of being swallowed below.
    void handleIo() throws IOException {
        try {
            riskyOperation();
        } catch (IOException e) {
            throw e;
        } catch (Exception e) {
            System.out.println("failed: " + e);
        }
    }

    // Multi-catch guard ahead of a broad handler.
    void handleMulti() throws IOException, SQLException {
        try {
            riskyOperation();
        } catch (IOException | SQLException e) {
            throw e;
        } catch (RuntimeException e) {
            System.out.println("runtime: " + e);
        } catch (Exception e) {
            System.out.println("failed: " + e);
        } finally {
            System.out.println("done");
        }
    }

    void riskyOperation() throws Exception {}
}
