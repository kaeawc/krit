// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Seeds whose value is not fixed: parameters, generated seeds, reassigned variables, and arrays that may have been
// filled before setSeed reads them.
package test.securerandomconstantseednegative

import java.security.SecureRandom

private var reassignedTopLevel = 1L

fun bump() {
    reassignedTopLevel++
}

fun parameter(seed: Long, bytes: ByteArray) {
    val rng = SecureRandom()
    rng.setSeed(seed)
    rng.setSeed(bytes)
}

fun generated() {
    val rng = SecureRandom()
    rng.setSeed(rng.generateSeed(8))
    val fresh = rng.generateSeed(16)
    rng.setSeed(fresh)
}

fun reassigned(flag: Boolean) {
    val rng = SecureRandom()
    var seed = 1L
    if (flag) seed = System.nanoTime()
    rng.setSeed(seed)
    rng.setSeed(reassignedTopLevel)
}

fun filledArray() {
    val rng = SecureRandom()
    val seed = ByteArray(16)
    rng.nextBytes(seed)
    rng.setSeed(seed)
}

fun filledInLoop() {
    val rng = SecureRandom()
    val seed = ByteArray(16)
    for (i in 0 until 2) {
        rng.setSeed(seed)
        rng.nextBytes(seed)
    }
}

fun nonConstantArithmetic(offset: Long) {
    val rng = SecureRandom()
    rng.setSeed(offset + 1)
    rng.setSeed(byteArrayOf(1, offset.toByte()))
}

fun textBytes() {
    val rng = SecureRandom()
    rng.setSeed("seed".toByteArray())
}

// The init lambda decides the contents, so the array is not known to be fixed.
fun initializedArray() {
    val rng = SecureRandom()
    rng.setSeed(ByteArray(8) { it.toByte() })
}
