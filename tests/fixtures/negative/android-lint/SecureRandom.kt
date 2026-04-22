package com.example

import java.security.SecureRandom
import java.util.concurrent.ThreadLocalRandom

class TokenGenerator {
    fun generateToken(): Long {
        val rng = SecureRandom()
        return rng.nextLong()
    }

    fun nextInt(): Int {
        return ThreadLocalRandom.current().nextInt()
    }

    fun comment() {
        // Random()
        val unrelated = "Random("
        println(unrelated)
    }
}
