package com.example

import java.security.SecureRandom

class TokenGenerator {
    fun generateToken(): Long {
        val rng = SecureRandom()
        return rng.nextLong()
    }
}
