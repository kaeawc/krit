// RENDER_DIAGNOSTICS_FULL_TEXT
// Deliberate precision differences from the Go rule. Go matches the IV
// argument by its source text: it reports any argument that starts with a
// string literal followed by `.toByteArray(`, and any argument whose text
// contains `Base64.decode(` and a quote anywhere. None of these IVs is built
// from literal bytes, so FIR reports none of them.
package test

import java.security.SecureRandom
import java.util.Base64 as JBase64
import javax.crypto.spec.IvParameterSpec

object Base64 {
    fun decode(text: String, flags: Int): ByteArray = JBase64.getDecoder().decode(text.reversed())
}

class Prefs {
    fun getString(key: String, default: String): String = key + default
}

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

    // Go reports: a lambda computes the IV from the literal.
    fun lambda() = IvParameterSpec("0123456789abcdef".toByteArray().let { xor(it, random) })

    // Go reports: the lambda overwrites the literal bytes with random ones.
    fun lambdaMutates() = IvParameterSpec("0123456789abcdef".toByteArray().also { SecureRandom().nextBytes(it) })

    // Go reports: the decoded text comes from preferences; the quote belongs to the key.
    fun decodedPreference() = IvParameterSpec(JBase64.getDecoder().decode(prefs.getString("iv", "")))

    // Go reports: the decoded literal is only an input to xor with random bytes.
    fun decodedMixed() = IvParameterSpec(xor(JBase64.getDecoder().decode("AAAA"), random))

    // Go reports: `Base64` here is the local object above, not a platform decoder.
    fun decodedLookalike() = IvParameterSpec(Base64.decode("AAAAAAAAAAAAAAAAAAAAAA==", 0))
}
