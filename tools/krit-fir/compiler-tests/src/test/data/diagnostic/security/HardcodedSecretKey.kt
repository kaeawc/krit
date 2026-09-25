// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: SecretKeySpec built from hardcoded key bytes, in every shape the Go
// rule reports.
package test

import java.util.Base64
import javax.crypto.spec.SecretKeySpec
import kotlin.io.encoding.ExperimentalEncodingApi

const val KEY_TEXT = "p@ssw0rd12345678"

@OptIn(ExperimentalEncodingApi::class, ExperimentalStdlibApi::class)
class Crypto(private val runtime: ByteArray) {
    // Literal byte arrays.
    fun byteArrays() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(1, 2, 3, 4), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(kotlin.byteArrayOf(0x7F, -1, 1_0), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(+1), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>((byteArrayOf(1, 2)), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(1, 2, 3, 4), 0, 2, "AES")
        <!HardcodedSecretKey!>javax<!>.crypto.spec.SecretKeySpec(byteArrayOf(1, 2), "AES")
    }

    // Bytes of a literal string.
    fun stringBytes() {
        <!HardcodedSecretKey!>SecretKeySpec<!>("p@ssw0rd12345678".toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("p@ssw0rd12345678".toByteArray(Charsets.UTF_8), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(("p@ssw0rd12345678".toByteArray()), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("""p@ssw0rd12345678""".toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("p@ssw0rd12345678".encodeToByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("00112233445566778899aabbccddeeff".hexToByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("key-$KEY_TEXT".toByteArray(), "AES")
    }

    // Go matches the conversion anywhere in an argument that starts with a
    // string literal, so later calls on the literal's bytes still count.
    fun chained() {
        <!HardcodedSecretKey!>SecretKeySpec<!>("p@ssw0rd".toByteArray().copyOf(16), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("p@ssw0rd".toByteArray() + runtime, "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("p@ssw0rd".let { "p@ssw0rd12345678".toByteArray() }, "AES")
    }

    // Base64 decoding of a hardcoded string, anywhere inside the key argument.
    fun decoded(debug: Boolean) {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(java.util.Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg=".toByteArray()), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(kotlin.io.encoding.Base64.decode("c2VjcmV0MTIzNDU2Nzg="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg=").copyOf(16), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(listOf(Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg=")).first(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(run { Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg=") }, "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("c2VjcmV0" + "MTIzNDU2Nzg="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("c2VjcmV0-MTIzNDU2Nzg=".replace("-", "")), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(if (debug) "ZGVidWc=" else "c2VjcmV0MTIzNDU2Nzg="), "AES")
    }

    // Reported on the line where the constructor call starts.
    fun multiline() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(
            byteArrayOf(1, 2, 3, 4),
            "AES",
        )
        <!HardcodedSecretKey!>javax<!>.crypto.spec
            .SecretKeySpec(byteArrayOf(1, 2), "AES")
    }

    // Members of an anonymous object and a lambda are checked too.
    fun nested(): Any {
        val keys = listOf(1).map { <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(1, 2), "AES") }
        return object {
            val key = <!HardcodedSecretKey!>SecretKeySpec<!>("p@ssw0rd12345678".toByteArray(), "AES")
            fun make() = <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(1), "AES")
        }
    }
}
