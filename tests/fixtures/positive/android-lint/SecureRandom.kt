package com.example

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
}
