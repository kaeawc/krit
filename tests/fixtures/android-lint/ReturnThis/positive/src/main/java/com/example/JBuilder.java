package com.example;

import androidx.annotation.ReturnThis;

public class JBuilder {
    @ReturnThis
    public JBuilder size(int size) {
        if (size < 0) {
            return null;
        }
        return new JBuilder();
    }
}
