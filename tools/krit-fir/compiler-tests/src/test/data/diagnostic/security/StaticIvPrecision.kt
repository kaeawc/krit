// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 32, 35, 38, 41, 44, 47, 50, 53, 56, 59, 62, 65, 68, 71, 74, 77
// Deliberate precision differences from the Go rule. Go matches the IV
// argument by its source text: it reports any argument that starts with a
// string literal followed by `.toByteArray(`, and any argument whose text
// contains `Base64.decode(` and a quote anywhere. None of these IVs is built
// from literal bytes: each one takes runtime or random bytes, so FIR reports
// none of them.
package test

import java.security.SecureRandom
import java.util.Base64 as JBase64
import javax.crypto.spec.IvParameterSpec

class Prefs {
    fun getString(key: String, default: String): String = System.getenv(key) ?: default
}

lateinit var LATE_PREFIX: String

val ENV_PREFIX: String
    get() = System.getenv("IV_PREFIX") ?: "0123456789"

fun ByteArray.randomized(rng: SecureRandom): ByteArray = ByteArray(size).also { rng.nextBytes(it) }

fun ByteArray.mixWith(other: ByteArray): ByteArray = ByteArray(size) { i -> (this[i].toInt() xor other[i].toInt()).toByte() }

fun xor(a: ByteArray, b: ByteArray): ByteArray = a.mixWith(b)

class Crypto(private val prefs: Prefs, private val random: ByteArray, private val nonce: String) {
    // Go reports: the template interpolates a runtime value.
    fun template() = IvParameterSpec("iv-$nonce".toByteArray())

    // Go reports: random bytes are appended to the literal ones.
    fun mixedPlus() = IvParameterSpec("01234567".toByteArray() + random)

    // Go reports: the literal is only an input to a function of random bytes.
    fun mixedCall() = IvParameterSpec("0123456789abcdef".toByteArray().mixWith(random))

    // Go reports: a collection of runtime bytes is appended to the literal ones.
    fun mixedCollection(extra: List<Byte>) = IvParameterSpec("01234567".toByteArray() + extra)

    // Go reports: a runtime byte is appended to the literal ones.
    fun mixedByte(extra: Byte) = IvParameterSpec("0123456789abcde".toByteArray() + extra)

    // Go reports: the lambda returns the literal xored with random bytes.
    fun lambda() = IvParameterSpec("0123456789abcdef".toByteArray().let { xor(it, random) })

    // Go reports: the lambda overwrites the literal bytes with random ones.
    fun lambdaMutates() = IvParameterSpec("0123456789abcdef".toByteArray().also { SecureRandom().nextBytes(it) })

    // Go reports: the decoded text comes from preferences; the quote belongs to the key.
    fun decodedPreference() = IvParameterSpec(JBase64.getDecoder().decode(prefs.getString("iv", "")))

    // Go reports: the decoded literal is only an input to xor with random bytes.
    fun decodedMixed() = IvParameterSpec(xor(JBase64.getDecoder().decode("AAAA"), random))

    // Go reports: the template reads a lateinit var, which has no initializer; its value is assigned at runtime.
    fun lateinitTemplate() = IvParameterSpec("$LATE_PREFIX-abcde".toByteArray())

    // Go reports: the template reads a getter that returns a runtime value.
    fun getterTemplate() = IvParameterSpec("$ENV_PREFIX-abcde".toByteArray())

    // Go reports: a random source is fed into the chain.
    fun randomArgument(rng: SecureRandom) = IvParameterSpec("0123456789abcdef".toByteArray().randomized(rng))

    // Go reports: the lambda overwrites the literal bytes with random ones.
    fun applyMutates() = IvParameterSpec("0123456789abcdef".toByteArray().apply { SecureRandom().nextBytes(this) })

    // Go reports: the lambda copies a random byte into the IV.
    fun lambdaSetsRandom() = IvParameterSpec("0123456789abcdef".toByteArray().also { it[0] = random[0] })

    // Go reports: the callable reference fills the IV with random bytes.
    fun referenceMutates() = IvParameterSpec("0123456789abcdef".toByteArray().also(SecureRandom()::nextBytes))

    // Go reports: the lambda appends random bytes to the literal ones.
    fun lambdaMixes() = IvParameterSpec("01234567".toByteArray().let { it + random })
}
