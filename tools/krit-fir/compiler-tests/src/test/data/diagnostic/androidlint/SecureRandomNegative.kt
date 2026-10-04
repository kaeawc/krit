// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import java.security.SecureRandom
import java.util.concurrent.ThreadLocalRandom

class Negatives {
    fun defaultSeeding(): Long {
        val rng = SecureRandom()
        return rng.nextLong()
    }

    fun threadLocal(): Int = ThreadLocalRandom.current().nextInt()

    fun byteArraySeed(seedBytes: ByteArray) {
        val rng = SecureRandom()
        rng.setSeed(seedBytes)
        rng.setSeed(byteArrayOf(1, 2, 3))
    }

    fun variableSeed(seed: Long) {
        val rng = SecureRandom()
        rng.setSeed(seed)
        rng.setSeed(seed + 1)
    }

    fun kotlinRandom(): Int {
        val rng = kotlin.random.Random(1234)
        return rng.nextInt() + kotlin.random.Random.nextInt()
    }

    // The seeded SecureRandom constructor belongs to the TrulyRandom rule.
    fun seededConstructor(): SecureRandom = SecureRandom(byteArrayOf(1))

    // A java.util.Random that is not a SecureRandom: the setSeed is not
    // SecureRandom.setSeed.
    fun plainRandomSeed(r: java.util.Random) {
        r.setSeed(1L)
    }

    fun threadLocalSeed() {
        ThreadLocalRandom.current().setSeed(1L)
    }

    fun comment() {
        // Random()
        // rng.setSeed(1L)
        val unrelated = "Random( setSeed(1L)"
        println(unrelated)
    }
}

// A superclass delegation and an object expression extending Random are not
// constructor calls; a subclass constructor is not java.util.Random.
class MyRandom : java.util.Random()

fun anonymous(): java.util.Random = object : java.util.Random() {}

fun subclass(): java.util.Random = MyRandom()

fun reference(): () -> java.util.Random = ::MyRandom
