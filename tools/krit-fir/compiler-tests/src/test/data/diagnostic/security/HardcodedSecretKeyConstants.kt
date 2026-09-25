// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: the key is read from a value the source fixes: a const val, a
// property (val or var, final or open) whose initializer is a hardcoded
// string, or a Java compile-time constant field.
package test

import android.Manifest
import java.util.Base64
import java.util.jar.JarFile
import javax.crypto.spec.SecretKeySpec

const val KEY_B64 = "c2VjcmV0MTIzNDU2Nzg"
private val KEY_VAL = "c2VjcmV0MTIzNDU2Nzg="
val NONCONST = "p@ssw0rd12345678"
val CHAINED = KEY_VAL
val LAZY_KEY by lazy { "c2VjcmV0MTIzNDU2Nzg=" }
val GETTER_KEY: String get() = "c2VjcmV0MTIzNDU2Nzg="

object Keys {
    val SECRET = "p@ssw0rd12345678"
    @JvmField val FIELD = "c2VjcmV0MTIzNDU2Nzg="
}

class Holder {
    val key = "c2VjcmV0MTIzNDU2Nzg="
}

var MUTABLE_KEY = "c2VjcmV0MTIzNDU2Nzg="

open class OpenKey {
    open val key = "c2VjcmV0MTIzNDU2Nzg="
}

class Crypto(private val holder: Holder) {
    // A String call on a const val with hardcoded arguments.
    fun constReads() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(KEY_B64 + "="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(KEY_B64.padEnd(20, '=') + ""), "AES")
    }

    // A final val with no custom getter (or a getter that returns a hardcoded
    // string), no delegate other than `lazy { hardcoded }`, and a hardcoded
    // initializer holds the same bytes as its literal.
    fun valReads() {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("$KEY_VAL"), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(KEY_VAL.trim() + ""), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("$NONCONST".toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("k-${Keys.SECRET}".toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("${Keys.FIELD}"), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("${holder.key}"), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("$CHAINED"), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("$LAZY_KEY"), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("$GETTER_KEY"), "AES")
        val local = "c2VjcmV0MTIzNDU2Nzg="
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("$local"), "AES")
    }

    // Java compile-time constants, from a Java source (the android.Manifest
    // stub) and from a JDK class file (JarFile.MANIFEST_NAME): the value is
    // compiled into the binary.
    fun javaConstants() {
        <!HardcodedSecretKey!>SecretKeySpec<!>("${Manifest.permission.CAMERA}".toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(Manifest.permission.CAMERA + "=="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("${JarFile.MANIFEST_NAME}".toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(JarFile.MANIFEST_NAME + "=="), "AES")
    }

    // A member or top-level var or an open val with a hardcoded initializer:
    // the secret is still written in the source (the default), even if the
    // value can later be reassigned or overridden. Go reports these because
    // the argument starts with or holds a quote, and FIR matches it. Local
    // vars are in HardcodedSecretKeyLocalVars.
    fun mutableOrOverridable(base: OpenKey) {
        <!HardcodedSecretKey!>SecretKeySpec<!>("$MUTABLE_KEY".toByteArray(), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("$MUTABLE_KEY"), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("${base.key}"), "AES")
    }

    // Go misses these because the key argument holds no quote; FIR is correct
    // because the decoded string is fixed by the source (for the var and the
    // open val, by its hardcoded initializer).
    fun bareReads(base: OpenKey) {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(MUTABLE_KEY), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(base.key), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(KEY_B64), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(KEY_VAL), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(Manifest.permission.CAMERA), "AES")
    }

    // A read that is not a string-to-bytes call on a leading literal is not
    // reported by Go, and FIR matches it.
    fun notLeadingLiteral() {
        SecretKeySpec(KEY_B64.toByteArray(), "AES")
        SecretKeySpec(Manifest.permission.CAMERA.toByteArray(), "AES")
    }
}
