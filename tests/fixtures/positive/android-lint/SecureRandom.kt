package com.example

class TokenGenerator {
    fun generateToken(): Long {
        val rng = java.util.Random()
        return rng.nextLong()
    }
}
