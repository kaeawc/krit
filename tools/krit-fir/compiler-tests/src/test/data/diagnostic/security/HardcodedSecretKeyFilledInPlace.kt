// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// A local key array that is filled or handed to a call before the key is built
// holds runtime bytes, whatever its initializer. Read-only uses keep it fixed.
package test

import java.security.SecureRandom
import javax.crypto.spec.SecretKeySpec

class FilledInPlace(private val random: SecureRandom) {
    fun random() {
        val key = "0123456789abcdef".toByteArray()
        random.nextBytes(key)
        SecretKeySpec(key, "AES")
    }

    fun zeroed() {
        val key = byteArrayOf(1, 2, 3, 4)
        key.fill(0)
        SecretKeySpec(key, "AES")
    }

    fun indexed(b: Byte) {
        val key = byteArrayOf(1, 2, 3, 4)
        key[0] = b
        SecretKeySpec(key, "AES")
    }

    fun readOnly() {
        val key = byteArrayOf(1, 2, 3, 4)
        check(key.size == 4)
        <!HardcodedSecretKey!>SecretKeySpec<!>(key, "AES")
    }

    fun filledAfter() {
        val key = byteArrayOf(1, 2, 3, 4)
        <!HardcodedSecretKey!>SecretKeySpec<!>(key, "AES")
        key.fill(0)
    }
}
