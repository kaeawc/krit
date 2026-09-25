// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: hardcoded key bytes written in forms the Go rule's text parser
// rejects.
package test

import java.util.Base64
import javax.crypto.spec.SecretKeySpec
import kotlin.io.encoding.ExperimentalEncodingApi

@OptIn(ExperimentalEncodingApi::class)
class Crypto {
    // Go misses these because it splits the argument text on commas and parses
    // each part as a plain decimal or hex number; FIR is correct because every
    // element is an integer literal.
    fun literalLists() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(-0x1, 0x7F), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(0b1010, 1), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf((1), 2), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(1, /* two */ 2), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(byteArrayOf(1, 2,), "AES")
    }

    // Go misses these because it looks for `".toByteArray(`,
    // `Base64.decode(`, and `Base64.getDecoder().decode(` as contiguous text;
    // FIR is correct because the calls are the same across whitespace or a
    // line break before the dot.
    fun whitespace() {
        <!HardcodedSecretKey!>SecretKeySpec<!>("p@ssw0rd12345678" .toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(kotlin.io.encoding.Base64 .decode("c2VjcmV0MTIzNDU2Nzg="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder() .decode("c2VjcmV0MTIzNDU2Nzg="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64 .getDecoder().decode("c2VjcmV0MTIzNDU2Nzg="), "AES")
    }

    fun lineBreaks() {
        <!HardcodedSecretKey!>SecretKeySpec<!>("p@ssw0rd12345678"
            .toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder()
            .decode("c2VjcmV0MTIzNDU2Nzg="), "AES")
    }
}
