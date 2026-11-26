package dev.krit.firsample.app

// Depends only on the Kotlin standard library: authoritative even with no
// build model at all.
fun greeting(name: String, visits: Int): String =
    if (visits <= 1) "Welcome, $name" else "Welcome back, $name ($visits visits)"
