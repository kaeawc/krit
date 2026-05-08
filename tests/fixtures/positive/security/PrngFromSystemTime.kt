package test

import java.util.Random
import javax.crypto.Cipher

class Crypto {
    fun rng() {
        Random(System.currentTimeMillis())
    }
}
