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

    fun byteArraySeed(seedBytes: ByteArray): Long {
        val rng = SecureRandom()
        rng.setSeed(seedBytes)
        return rng.nextLong()
    }

    fun kotlinRandom(): Int {
        val rng = kotlin.random.Random(1234)
        return rng.nextInt()
    }

    fun comment() {
        // Random()
        val unrelated = "Random("
        println(unrelated)
    }
}
