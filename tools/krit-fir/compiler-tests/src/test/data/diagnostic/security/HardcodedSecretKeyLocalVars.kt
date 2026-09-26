// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 23, 45, 101, 102, 105
// A local var holds a hardcoded key only when its initializer and every value
// its function assigns to it are hardcoded: any assignment (before the read,
// after it in a loop, or inside a lambda or local function that captures the
// var) may be the value read. A compound assignment (`key += x`) combines the
// current value with `x`, so only `x` is checked.
package test

import java.util.Base64
import javax.crypto.spec.SecretKeySpec

const val OTHER_KEY = "b3RoZXJrZXkxMjM0NTY3OA=="

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

    // Reassigned only with hardcoded values: another literal, a const, a
    // hardcoded suffix, or the var itself extended with a literal. Every value
    // the var can hold is written in the source. Go reports the templates and
    // misses the bare reads; FIR reports all of them.
    fun reassignedWithHardcodedValues(flag: Boolean) {
        var key = "c2VjcmV0MTIzNDU2Nzg="
        if (flag) key = "b3RoZXJrZXkxMjM0NTY3OA=="
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(key), "AES")
        var fromConst = "c2VjcmV0MTIzNDU2Nzg="
        if (flag) fromConst = OTHER_KEY
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(fromConst), "AES")
        var suffixed = "c2VjcmV0MTIzNDU2Nzg"
        suffixed += "="
        <!HardcodedSecretKey!>SecretKeySpec<!>("$suffixed".toByteArray(), "AES")
        var extended = "c2VjcmV0MTIzNDU2Nzg"
        extended = extended + "="
        <!HardcodedSecretKey!>SecretKeySpec<!>(Base64.getDecoder().decode(extended), "AES")
    }

    // Neither Go nor FIR reports these bare reads: some assigned value is a
    // runtime value.
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

    fun compoundRuntimeOperand() {
        var key = "c2VjcmV0"
        key += loadKey()
        SecretKeySpec(Base64.getDecoder().decode(key), "AES")
    }

    // A literal reassignment does not make up for a runtime one.
    fun mixedReassignments(flag: Boolean) {
        var key = "c2VjcmV0MTIzNDU2Nzg="
        if (flag) key = "b3RoZXJrZXkxMjM0NTY3OA==" else key = loadKey()
        SecretKeySpec(Base64.getDecoder().decode(key), "AES")
    }

    // Go reports these because the argument starts with or holds a quote;
    // FIR is correct to drop them: the var is also assigned a runtime value,
    // so the bytes read need not be a literal.
    fun reassignedTemplates(flag: Boolean) {
        var key = "c2VjcmV0MTIzNDU2Nzg="
        key = loadKey()
        SecretKeySpec("$key".toByteArray(), "AES")
        SecretKeySpec(Base64.getDecoder().decode("$key"), "AES")
        var mixed = "c2VjcmV0MTIzNDU2Nzg="
        if (flag) mixed = "b3RoZXJrZXkxMjM0NTY3OA==" else mixed = loadKey()
        SecretKeySpec("$mixed".toByteArray(), "AES")
    }
}
