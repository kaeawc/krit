package test

import java.security.SecureRandom
import java.util.Random

class Crypto {
    fun rng(seed: Long) {
        SecureRandom()
        Random(seed)
    }
}
