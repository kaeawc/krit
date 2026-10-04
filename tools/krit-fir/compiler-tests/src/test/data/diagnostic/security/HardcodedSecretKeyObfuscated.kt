// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 17, 18, 19, 20, 24, 25, 26, 34, 35, 36
// Positive: a decode input built only from source literals, through calls
// whose intermediate values are not Strings, casts, and String or
// StringBuilder constructors and charArrayOf with hardcoded arguments. Every
// byte of the key comes from the source, as Go reports.
package test

import java.util.Base64
import javax.crypto.spec.SecretKeySpec

fun loadKey(name: String): String = name

class Crypto {
    fun obfuscated() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(StringBuilder("=gzN2UDNzITM0VmcjV2c").reverse().toString()), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(String("c2VjcmV0MTIzNDU2Nzg=".toCharArray())), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(charArrayOf('Y', 'Q', '=', '=').concatToString() + ""), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg=".reversed().toString()), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(String("c2VjcmV0MTIzNDU2Nzg=".toByteArray(Charsets.UTF_8), Charsets.UTF_8)), "AES")
    }

    fun casts(any: Any) {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg=" as String), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(("c2VjcmV0MTIzNDU2Nzg=") as CharSequence as String), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(("c2VjcmV0MTIzNDU2Nzg=" as? String)!!), "AES")
    }

    // Go reports these because the argument holds a quote; FIR is correct
    // because the literal is only the name of a value looked up at runtime: a
    // call on a class qualifier or with no receiver does not count on its
    // literal arguments alone.
    fun lookups() {
        SecretKeySpec(Base64.getDecoder().decode(System.getenv("K")), "AES")
        SecretKeySpec(Base64.getDecoder().decode(loadKey("name")), "AES")
        SecretKeySpec(Base64.getDecoder().decode(StringBuilder(loadKey("name")).toString()), "AES")
    }
}
