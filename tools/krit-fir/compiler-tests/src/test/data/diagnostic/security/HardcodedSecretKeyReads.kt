// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// The key is the bytes of a value the source fixes, with no literal written in
// the argument: Go reads only a leading string literal or a byteArrayOf list of
// plain literals, so it misses all of these. The negatives read runtime data.
package test

import javax.crypto.spec.SecretKeySpec

private const val KEY_TEXT = "p@ssw0rd12345678"
private val KEY_VAL = "p@ssw0rd12345678"
private val KEY_BYTES = byteArrayOf(0x2b, 0x7e, 0x15, 0x16)
private val TEXT_BYTES = "p@ssw0rd12345678".toByteArray()
private val CONVERTED = byteArrayOf(1.toByte(), 2)
private val FROM_ENV: String = System.getenv("KEY")
private val RUNTIME_BYTES = ByteArray(16)

object Keys {
    const val SECRET = "p@ssw0rd12345678"
}

class Reads(private val runtime: String, private val bytes: ByteArray) {
    fun hardcodedReads() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(KEY_TEXT.toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(KEY_TEXT.toByteArray(Charsets.UTF_8), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(KEY_TEXT.encodeToByteArray(), 0, 16, "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(KEY_VAL.toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Keys.SECRET.toByteArray().copyOf(16), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(KEY_BYTES, "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(TEXT_BYTES, "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(CONVERTED, "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(1.toByte(), 2), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(1, 2).copyOf(16), "AES")
        val local = "p@ssw0rd12345678"
        <!HardcodedSecretKey!>SecretKeySpec<!>(local.toByteArray(), "AES")
    }

    fun runtimeReads(param: String) {
        SecretKeySpec(runtime.toByteArray(), "AES")
        SecretKeySpec(param.toByteArray(Charsets.UTF_8), "AES")
        SecretKeySpec(FROM_ENV.toByteArray(), "AES")
        SecretKeySpec(bytes, "AES")
        SecretKeySpec(RUNTIME_BYTES, "AES")
        SecretKeySpec(ByteArray(16), "AES")
        SecretKeySpec(byteArrayOf(1, param.length.toByte()), "AES")
        SecretKeySpec(runtime.toByteArray().copyOf(16), "AES")
        SecretKeySpec(KEY_TEXT.toByteArray().copyOf(param.length), "AES")
    }
}
