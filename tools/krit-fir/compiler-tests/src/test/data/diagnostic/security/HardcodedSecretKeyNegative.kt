// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 53, 54, 55, 56, 57, 64, 65, 79, 80, 81, 82
// Negative: key bytes that are not hardcoded, and shapes the Go rule does not
// recognize, must NOT trigger HardcodedSecretKey.
package test

import java.io.File
import java.security.KeyStore
import java.util.Base64
import javax.crypto.SecretKey
import javax.crypto.spec.SecretKeySpec

const val KEY_TEXT = "p@ssw0rd12345678"
val FROM_ENV: String = System.getenv("KEY_B64")
lateinit var LATE_KEY: String

class RuntimeKey(seed: String) {
    val key = seed
}

class Crypto(private val keyStore: KeyStore, private val prefs: Map<String, String>) {
    fun runtimeKeys(param: ByteArray, text: String) {
        SecretKeySpec(param, "AES")
        SecretKeySpec(param.copyOf(16), "AES")
        SecretKeySpec(text.toByteArray(), "AES")
        SecretKeySpec(Base64.getDecoder().decode(text), "AES")
        keyStore.getKey("alias", null) as SecretKey
    }

    // Not a literal list: empty, a converted value, or a spread.
    fun notLiteralLists(param: ByteArray) {
        SecretKeySpec(byteArrayOf(), "AES")
        SecretKeySpec(byteArrayOf(1.toByte(), 2), "AES")
        SecretKeySpec(byteArrayOf(*param), "AES")
        SecretKeySpec(byteArrayOf(1, 2).copyOf(16), "AES")
    }

    // Go only reads the literal bytes when the argument starts with the
    // string literal (or is the byteArrayOf call itself).
    fun notLeadingLiteral(param: ByteArray) {
        SecretKeySpec(KEY_TEXT.toByteArray(), "AES")
        SecretKeySpec(param + "p@ssw0rd12345678".toByteArray(), "AES")
        SecretKeySpec("p@ssw0rd12345678"?.toByteArray()!!, "AES")
        SecretKeySpec(Base64.getMimeDecoder().decode("c2VjcmV0MTIzNDU2Nzg="), "AES")
    }

    // Go reports these because the argument starts with a quote; FIR is
    // correct because the template interpolates a runtime value (a parameter,
    // a local or property initialized from one), so the key bytes are not
    // hardcoded.
    fun templates(pin: String, runtime: RuntimeKey) {
        val local = pin.trim()
        SecretKeySpec("$pin".toByteArray(), "AES")
        SecretKeySpec("salt-$pin".toByteArray(), "AES")
        SecretKeySpec("$local".toByteArray(), "AES")
        SecretKeySpec(Base64.getDecoder().decode("${runtime.key}"), "AES")
        SecretKeySpec(Base64.getDecoder().decode("$FROM_ENV"), "AES")
    }

    // Go reports these because the argument starts with or holds a quote;
    // FIR is correct because a lateinit var has no initializer: its value is
    // assigned at runtime, so the key bytes are not hardcoded.
    fun lateinitTemplates() {
        SecretKeySpec("$LATE_KEY".toByteArray(), "AES")
        SecretKeySpec(Base64.getDecoder().decode("$LATE_KEY"), "AES")
    }

    // Neither Go nor FIR reports these.
    fun bareRuntimeReads() {
        SecretKeySpec(Base64.getDecoder().decode(FROM_ENV), "AES")
        SecretKeySpec(Base64.getDecoder().decode(LATE_KEY), "AES")
    }

    // Go reports these because the argument contains `.decode(` on a Base64
    // decoder and a quote anywhere; FIR is correct because the decoded string
    // is a runtime value: the literal is a lookup key or only part of the
    // input.
    fun decodedLookups(pin: String) {
        SecretKeySpec(Base64.getDecoder().decode(prefs["key"]), "AES")
        SecretKeySpec(Base64.getDecoder().decode(prefs.getValue("key")), "AES")
        SecretKeySpec(Base64.getDecoder().decode("c2VjcmV0" + pin), "AES")
        SecretKeySpec(Base64.getDecoder().decode(File.separator + "=="), "AES")
    }

    // A subclass instance is not a direct SecretKeySpec call, as in Go.
    fun subclass(): SecretKey = object : SecretKeySpec(byteArrayOf(1, 2), "AES") {}
}
