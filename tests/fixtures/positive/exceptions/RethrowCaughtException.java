package com.example.exceptions;

import java.io.IOException;

class Handler {
    void handle() throws Exception {
        try {
            riskyOperation();
        } catch (Exception e) {
            throw e;
        }
    }

    // The rethrow-only catch is the last clause, so it is still useless.
    void handleLast() throws Exception {
        try {
            riskyOperation();
        } catch (IOException e) {
            System.out.println("io failure: " + e);
        } catch (Exception e) {
            throw e;
        }
    }

    void riskyOperation() throws Exception {}
}
