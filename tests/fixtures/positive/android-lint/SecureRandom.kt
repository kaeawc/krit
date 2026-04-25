package com.example

import java.security.SecureRandom
import java.util.Random

class TokenGenerator {
    fun generateToken(): Long {
        val rng = Random()
        return rng.nextLong()
    }

    fun generateSeeded(): Long {
        val fq = java.util.Random(42L)
        return fq.nextLong()
    }

    fun deterministicSeed(): Long {
        val rng = SecureRandom()
        rng.setSeed(1234L)
        return rng.nextLong()
    }

    fun timeSeed(): Long {
        val rng = SecureRandom()
        rng.setSeed(System.currentTimeMillis())
        return rng.nextLong()
    }
}
