// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 17, 21, 25, 31, 36, 41, 46, 52, 56, 60, 61, 67, 74, 79, 87, 91
package test

import java.security.SecureRandom
import java.util.Random

class TokenGenerator {
    private val member: SecureRandom = SecureRandom()

    fun generateToken(): Long {
        val rng = <!SecureRandom!>Random()<!>
        return rng.nextLong()
    }

    fun generateSeeded(): Long {
        val fq = <!SecureRandom!>java.util.Random(42L)<!>
        return fq.nextLong()
    }

    fun chained(): Int = <!SecureRandom!>Random(7L)<!>.nextInt()

    fun deterministicSeed(): Long {
        val rng = SecureRandom()
        <!SecureRandom!>rng.setSeed(1234L)<!>
        return rng.nextLong()
    }

    fun intLiteralSeed() {
        val rng = SecureRandom()
        <!SecureRandom!>rng.setSeed(1234)<!>
    }

    fun parenthesizedSeed() {
        val rng = SecureRandom()
        <!SecureRandom!>rng.setSeed((99L))<!>
    }

    fun hexSeed() {
        val rng = SecureRandom()
        <!SecureRandom!>rng.setSeed(0x1F2EL)<!>
    }

    fun timeSeed(): Long {
        val rng = SecureRandom()
        <!SecureRandom!>rng.setSeed(System.currentTimeMillis())<!>
        return rng.nextLong()
    }

    fun nanoSeed() {
        val rng: SecureRandom = SecureRandom()
        <!SecureRandom!>rng.setSeed(System.nanoTime())<!>
    }

    fun memberSeed() {
        <!SecureRandom!>member.setSeed(5L)<!>
    }

    fun constructedReceiver() {
        <!SecureRandom!>SecureRandom().setSeed(3L)<!>
        <!SecureRandom!>java.security.SecureRandom().setSeed(4L)<!>
    }

    // An unnecessary safe call on a SecureRandom local.
    fun safeCall() {
        val local = SecureRandom()
        <!SecureRandom!>local?.setSeed(8L)<!>
    }

    // The variable's declared type is java.util.Random, but it holds a
    // SecureRandom, so setSeed dispatches to SecureRandom.setSeed.
    fun randomTypedSecureRandom() {
        val upcast: Random = SecureRandom()
        <!SecureRandom!>upcast.setSeed(11L)<!>
    }

    fun multiLineReceiver() {
        val rng = SecureRandom()
        <!SecureRandom!>rng<!>
            .setSeed(12L)
    }

    // A safe call keeps the selector as its source; the finding still sits on
    // the receiver's line, where Go reports the call expression.
    fun multiLineSafeCall() {
        val local = SecureRandom()
        <!SecureRandom!>local<!>
            ?.setSeed(13L)
    }

    fun multiLinePackageQualifier(): java.util.Random = <!SecureRandom!>java<!>
        .util.Random()
}
