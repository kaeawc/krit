// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 31, 36, 37, 38, 43, 44, 56, 72, 79, 80, 81, 87, 89, 91, 96, 97, 99, 101, 105
// Seeds that Lint's ConstantEvaluator folds to a fixed value, as Go's constant-seed check does: constants, vals and
// unassigned vars that hold one, arithmetic and conversions of them, Long.MAX_VALUE style limits, and arrays built
// from constants. Each is a fixed seed on a SecureRandom.
package test.securerandomconstantseed

import java.security.SecureRandom

const val SEED = 42L
private const val INT_SEED = 7
private val topLevelSeed = 9L
private var unassignedTopLevel = 10L
private val seedBytes = byteArrayOf(1, 2, 3)

object Seeds {
    const val FIXED = 11L
}

class Holder {
    private val memberSeed = 12L

    fun memberVal() {
        val rng = SecureRandom()
        <!SecureRandom!>rng.setSeed(memberSeed)<!>
    }
}

fun constant() {
    val rng = SecureRandom()
    <!SecureRandom!>rng.setSeed(SEED)<!>
}

fun constantArithmetic() {
    val rng = SecureRandom()
    <!SecureRandom!>rng.setSeed(SEED + 1)<!>
    <!SecureRandom!>rng.setSeed(SEED * 3L)<!>
    <!SecureRandom!>rng.setSeed(-SEED)<!>
}

fun conversion() {
    val rng = SecureRandom()
    <!SecureRandom!>rng.setSeed(INT_SEED.toLong())<!>
    <!SecureRandom!>rng.setSeed(1.toLong())<!>
}

// Go misses this: it follows only a bare name, and `Seeds.FIXED` is a qualified one.
fun qualifiedConstant() {
    val rng = SecureRandom()
    <!SecureRandom!>rng.setSeed(Seeds.FIXED)<!>
}

// Go matches the limits by their spelling, so it misses the fully qualified one.
fun limits() {
    val rng = SecureRandom()
    <!SecureRandom!>rng.setSeed(Long.MAX_VALUE)<!>
    <!SecureRandom!>rng.setSeed(java.lang.Long.MIN_VALUE)<!>
}

// Go does not fold infix bit operators; the value is still a constant.
fun bitOperators() {
    val rng = SecureRandom()
    <!SecureRandom!>rng.setSeed(SEED or 1L)<!>
    <!SecureRandom!>rng.setSeed(1L shl 4)<!>
}

// The first pass seeds the zeroed array; the lambda is not a loop to Go.
fun filledLaterInLambda() {
    val rng = SecureRandom()
    val seed = ByteArray(16)
    repeat(2) {
        <!SecureRandom!>rng.setSeed(seed)<!>
        rng.nextBytes(seed)
    }
}

fun topLevelValues() {
    val rng = SecureRandom()
    <!SecureRandom!>rng.setSeed(topLevelSeed)<!>
    <!SecureRandom!>rng.setSeed(unassignedTopLevel)<!>
    <!SecureRandom!>rng.setSeed(seedBytes)<!>
}

fun localValues() {
    val rng = SecureRandom()
    val seed = 5L
    <!SecureRandom!>rng.setSeed(seed)<!>
    var unassigned = 6L
    <!SecureRandom!>rng.setSeed(unassigned)<!>
    val alias = seed
    <!SecureRandom!>rng.setSeed(alias)<!>
}

fun arrays() {
    val rng = SecureRandom()
    <!SecureRandom!>rng.setSeed(byteArrayOf(1, 2, 3))<!>
    <!SecureRandom!>rng.setSeed(ByteArray(16))<!>
    val zeros = ByteArray(8)
    <!SecureRandom!>rng.setSeed(zeros)<!>
    // Another setSeed only reads the array.
    <!SecureRandom!>rng.setSeed(zeros)<!>
}

fun constructedReceiver() {
    <!SecureRandom!>SecureRandom().setSeed(SEED)<!>
}
