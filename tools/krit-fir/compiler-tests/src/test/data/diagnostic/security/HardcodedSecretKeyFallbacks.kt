// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 16, 17, 18, 19, 24, 25, 26, 27, 28, 33, 34, 35, 42, 43
// Positive: a hardcoded fallback or default key. The decoded value is the
// literal whenever the lookup misses (first launch, cleared prefs), so the key
// is hardcoded on that path, as Go reports.
package test

import android.content.SharedPreferences
import java.util.Base64
import javax.crypto.spec.SecretKeySpec

class Crypto(private val prefs: Map<String, String>, private val shared: SharedPreferences) {
    // The elvis right side, and any branch of an if, when, or try.
    fun branches(s: String?, debug: Boolean, remote: String) {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(prefs["key"] ?: "c2VjcmV0MTIzNDU2Nzg="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(s ?: "c2VjcmV0MTIzNDU2Nzg="), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(if (debug) "c2VjcmV0MTIzNDU2Nzg=" else remote), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(when { debug -> remote; else -> "c2VjcmV0MTIzNDU2Nzg=" }), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(try { remote.trim() } catch (e: Exception) { "c2VjcmV0MTIzNDU2Nzg=" }), "AES")
    }

    // The default value of a lookup, or the result of a fallback lambda.
    fun defaults(s: String?, remote: String) {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(prefs.getOrDefault("key", "c2VjcmV0MTIzNDU2Nzg=")), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(prefs.getOrElse("key") { "c2VjcmV0MTIzNDU2Nzg=" }), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(shared.getString("key", "c2VjcmV0MTIzNDU2Nzg=")), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(s!!.ifEmpty { "c2VjcmV0MTIzNDU2Nzg=" }), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(remote.ifBlank { "c2VjcmV0MTIzNDU2Nzg=" }), "AES")
    }

    // Scope functions whose value is the lambda's hardcoded result.
    fun scopes(remote: String) {
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(run { "c2VjcmV0MTIzNDU2Nzg=" }), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(remote.let { "c2VjcmV0MTIzNDU2Nzg=" }), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode("c2VjcmV0MTIzNDU2Nzg=".also { println(it) }), "AES")
    }

    // Go reports these because the argument holds a quote (the lookup key);
    // FIR is correct because no branch is hardcoded: a missing value is null,
    // not a hardcoded key.
    fun noHardcodedBranch(remote: String) {
        SecretKeySpec(Base64.getDecoder().decode(shared.getString("key", null)), "AES")
        SecretKeySpec(Base64.getDecoder().decode(prefs["key"] ?: remote), "AES")
    }
}
