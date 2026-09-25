// RENDER_DIAGNOSTICS_FULL_TEXT
// A SecretKeySpec lookalike never reports; same-named byte helpers still do,
// as in Go.
package test

import javax.crypto.spec.*
import test.KeyHolder as SecretKeySpec

class KeyHolder(val bytes: ByteArray, val algorithm: String)

// Local helpers named like the stdlib and Android ones Go matches by name.
fun byteArrayOf(vararg values: Int): ByteArray = ByteArray(values.size) { values[it].toByte() }

fun String.getBytes(): ByteArray = encodeToByteArray()

val String.bytes: ByteArray
    @JvmName("literalBytes") get() = encodeToByteArray()

object LegacyBase64 {
    fun decode(text: String, flags: Int): ByteArray = text.encodeToByteArray()
}

class Crypto {
    // Go reports this because it sees the javax.crypto.spec star import and no
    // declaration named SecretKeySpec; FIR is correct because `SecretKeySpec`
    // is an import alias of KeyHolder.
    fun aliasedLookalike() = SecretKeySpec(kotlin.byteArrayOf(1, 2, 3, 4), "AES")

    // The real class through its qualified name, fed by same-named local
    // helpers: Go matches the helpers by name, and so does FIR, because the
    // result is still built from the literal.
    fun realClassLocalHelpers() {
        <!HardcodedSecretKey!>javax<!>.crypto.spec.SecretKeySpec(byteArrayOf(1, 2, 3, 4), "AES")
        <!HardcodedSecretKey!>javax<!>.crypto.spec.SecretKeySpec("p@ssw0rd12345678".getBytes(), "AES")
        <!HardcodedSecretKey!>javax<!>.crypto.spec.SecretKeySpec(LegacyBase64.decode("c2VjcmV0MTIzNDU2Nzg=", 0), "AES")
        <!HardcodedSecretKey!>javax<!>.crypto.spec.SecretKeySpec("p@ssw0rd12345678".bytes, "AES")
    }

    // Go matches `".bytes` only at the end of the key argument, so a later
    // call on the bytes is not reported, by Go or by FIR.
    fun bytesThenCall() = javax.crypto.spec.SecretKeySpec("p@ssw0rd".bytes.copyOf(16), "AES")
}
