package com.example;

public class Endpoints {
    static final String NO_PASSWORD = "https://user@example.com/path";
    static final String FORMATTED = "ftp://deploy:%1$s@mirror.example.com";
    String concat(String password) {
        return "http://admin:" + password + "@db.example.com";
    }
}
