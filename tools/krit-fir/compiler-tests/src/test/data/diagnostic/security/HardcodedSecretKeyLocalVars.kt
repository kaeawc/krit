// RENDER_DIAGNOSTICS_FULL_TEXT
// A local var holds its hardcoded initializer only when nothing in its
// function assigns it again: a reassignment before the read, after it (a loop
// or a later call can observe it), or inside a lambda or local function that
// captures the var can replace the literal with a runtime value.
package test

import java.util.Base64
import javax.crypto.spec.SecretKeySpec

fun loadKey(): String = System.getenv("KEY") ?: ""

class Crypto {
    // Never reassigned: the key is the literal. Go misses the bare read (the
    // argument holds no quote) and reports the template; FIR reports both.
    fun neverReassigned() {
        var key = "c2VjcmV0MTIzNDU2Nzg="
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(key), "AES")
        <!HardcodedSecretKey!>SecretKeySpec<!>("$key".toByteArray(), "AES")
    }

    // Never reassigned, read inside a lambda: still the literal.
    fun neverReassignedReadInLambda(values: List<Int>) {
        var key = "c2VjcmV0MTIzNDU2Nzg="
        values.forEach { _ -> <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(key), "AES") }
    }

    // Neither Go nor FIR reports these bare reads of a reassigned var.
    fun reassignedBefore() {
        var key = ""
        key = loadKey()
        SecretKeySpec(Base64.getDecoder().decode(key), "AES")
    }

    fun reassignedAfter(times: Int) {
        var key = "c2VjcmV0MTIzNDU2Nzg="
        repeat(times) {
            SecretKeySpec(Base64.getDecoder().decode(key), "AES")
        }
        key = loadKey()
    }

    fun reassignedInLambda(values: List<String>) {
        var key = "c2VjcmV0MTIzNDU2Nzg="
        values.forEach { key = it }
        SecretKeySpec(Base64.getDecoder().decode(key), "AES")
    }

    fun reassignedInLocalFunction() {
        var key = "c2VjcmV0MTIzNDU2Nzg="
        fun reset() {
            key = loadKey()
        }
        reset()
        SecretKeySpec(Base64.getDecoder().decode(key), "AES")
    }

    fun compoundAssignment() {
        var key = "c2VjcmV0"
        key += loadKey()
        SecretKeySpec(Base64.getDecoder().decode(key), "AES")
    }

    // Go reports these because the argument starts with or holds a quote;
    // FIR is correct to drop them: the var is reassigned in its function, so
    // the bytes read need not be the literal. (Even a reassignment to another
    // literal drops the finding: only the initializer is trusted.)
    fun reassignedTemplates() {
        var key = "c2VjcmV0MTIzNDU2Nzg="
        key = loadKey()
        SecretKeySpec("$key".toByteArray(), "AES")
        SecretKeySpec(Base64.getDecoder().decode("$key"), "AES")
        var suffixed = "c2VjcmV0MTIzNDU2Nzg="
        suffixed += ""
        SecretKeySpec("$suffixed".toByteArray(), "AES")
    }
}
