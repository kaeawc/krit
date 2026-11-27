package com.example;

class JCrash {
    void install() {
        Thread.setDefaultUncaughtExceptionHandler((t, e) -> { });
    }
}
