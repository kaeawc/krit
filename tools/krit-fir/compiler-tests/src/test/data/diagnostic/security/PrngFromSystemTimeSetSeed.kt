// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: a java.util.Random seeded from a clock in a way Go
// does not match. Go only reads the first argument of a call spelled
// `Random(...)` and matches the clock by text, so it misses a setSeed call, a
// clock read stored in a local val first, and the Android uptime clocks. FIR
// is correct to report them because each seed is a clock value, which an
// attacker can guess.
package test

import android.os.SystemClock
import java.security.SecureRandom
import java.util.Random

class SeededRandom : Random()

class Seeds(private val fixed: Long) {
    fun setSeed(random: Random) {
        <!PrngFromSystemTime!>random.setSeed(System.currentTimeMillis())<!>
    }

    fun applySetSeed(): Random = Random().apply { <!PrngFromSystemTime!>setSeed(System.nanoTime())<!> }

    fun subclassSetSeed(random: SeededRandom) {
        <!PrngFromSystemTime!>random.setSeed(System.nanoTime() xor fixed)<!>
    }

    fun localVal(): Random {
        val seed = System.currentTimeMillis()
        return <!PrngFromSystemTime!>Random(seed)<!>
    }

    fun localValChain(random: Random) {
        val now = System.nanoTime()
        val seed = now * 31
        <!PrngFromSystemTime!>random.setSeed(seed)<!>
    }

    fun elapsedRealtime(): Random = <!PrngFromSystemTime!>Random(SystemClock.elapsedRealtime())<!>

    fun elapsedRealtimeNanos(): Random = <!PrngFromSystemTime!>Random(SystemClock.elapsedRealtimeNanos())<!>

    fun uptime(): Random = <!PrngFromSystemTime!>Random(SystemClock.uptimeMillis())<!>

    fun threadTime(): Random = <!PrngFromSystemTime!>Random(SystemClock.currentThreadTimeMillis())<!>

    // Not a clock seed: a fixed value, a var that may hold anything, and
    // SecureRandom, whose setSeed adds to its seed instead of replacing it.
    fun negatives(random: Random, secure: SecureRandom, input: Long) {
        random.setSeed(fixed)
        var seed = System.currentTimeMillis()
        seed = input
        random.setSeed(seed)
        secure.setSeed(System.currentTimeMillis())
        SecureRandom().setSeed(System.nanoTime())
    }
}
