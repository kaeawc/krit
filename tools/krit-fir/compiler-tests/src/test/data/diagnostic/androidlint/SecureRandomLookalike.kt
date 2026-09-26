// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 18, 30, 45, 46, 61, 78
package test

import java.security.SecureRandom
import java.util.Random

// A real finding, so the file is not vacuous.
fun real(): Random = <!SecureRandom!>Random()<!>

// Divergence (precision): a nested class named Random shadows the
// java.util.Random import inside Factory. Go reports the call because the
// file imports java.util.Random; the call constructs Factory.Random, not
// java.util.Random, so FIR does not report it.
class Factory {
    class Random(val seed: Long)

    fun make(): Any = Random(1L)
}

// Divergence (precision): a local class named SecureRandom with its own
// setSeed. Go reports the setSeed because the receiver's property is declared
// with a type spelled `SecureRandom`; it is the local class, not
// java.security.SecureRandom, so FIR does not report it.
fun localSecureRandom() {
    class SecureRandom {
        fun setSeed(seed: Long) = seed
    }
    val fake: SecureRandom = SecureRandom()
    fake.setSeed(1L)
}

// Divergence (precision): Go matches the receiver name against every property
// in the file. `shared` names a SecureRandom here...
class Owner {
    private val shared = SecureRandom()

    fun use(): Int = shared.nextInt()
}

// ...and a java.util.Random local here. Go reports this setSeed through the
// other declaration; the receiver is a java.util.Random constructed as one,
// not a SecureRandom, so FIR reports only the Random constructor.
fun sameName() {
    val shared = <!SecureRandom!>Random()<!>
    shared.setSeed(2L)
}

// Divergence (precision): a local object named System shadows java.lang.System.
// Go reports the setSeed because the argument is spelled
// `System.currentTimeMillis()`; it is the local function, which returns a
// SecureRandom value, neither a fixed nor a time-based seed, so FIR does not
// report it.
object Clock {
    object System {
        fun currentTimeMillis(): Long = SecureRandom().nextLong()
    }

    fun seed() {
        val rng = SecureRandom()
        rng.setSeed(System.currentTimeMillis())
    }
}

// A project extension named setSeed on an unrelated type.
class Seeder

fun Seeder.setSeed(seed: Long): Long = seed

fun extension(seeder: Seeder) {
    seeder.setSeed(3L)
}

// Divergence (precision): a java.util.Random parameter that shares the name of
// Owner's SecureRandom property. Go reports it through that property; the
// receiver is not known to be a SecureRandom, so FIR does not report it.
fun parameterShadow(shared: Random) {
    shared.setSeed(4L)
}
