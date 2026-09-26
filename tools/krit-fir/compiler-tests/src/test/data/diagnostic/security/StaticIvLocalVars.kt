// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 29, 35, 41, 51, 57, 65, 74, 80, 87
// An IV template over a local var holds literal bytes only when the var's
// initializer and every value its function assigns to it are literal: any
// assignment (before the read, after it, or inside a lambda or local
// function) may be the value read. A compound assignment (`prefix += x`)
// combines the current value with `x`, so only `x` is checked.
package test

import javax.crypto.spec.IvParameterSpec

const val OTHER_PREFIX = "9876543210"

fun loadPrefix(): String = System.getenv("IV_PREFIX") ?: ""

class Crypto {
    // Go reports each of these (a string literal followed by
    // `.toByteArray(`), and FIR matches it: the var is never reassigned, or
    // only with literal values (another literal, a const, a literal suffix).
    fun neverReassigned(): IvParameterSpec {
        var prefix = "0123456789"
        return <!StaticIv!>IvParameterSpec("$prefix-abcde".toByteArray())<!>
    }

    fun reassignedWithLiterals(flag: Boolean): IvParameterSpec {
        var prefix = "0123456789"
        if (flag) prefix = "abcdefghij"
        if (flag) prefix = OTHER_PREFIX
        return <!StaticIv!>IvParameterSpec("$prefix-abcde".toByteArray())<!>
    }

    fun compoundLiteral(): IvParameterSpec {
        var prefix = "0123456789"
        prefix += "ab"
        return <!StaticIv!>IvParameterSpec("$prefix-abcd".toByteArray())<!>
    }

    fun selfTemplate(): IvParameterSpec {
        var prefix = "01234"
        prefix = "$prefix-5678"
        return <!StaticIv!>IvParameterSpec("$prefix-abcde".toByteArray())<!>
    }

    // Go reports each of these because the argument starts with a string
    // literal followed by `.toByteArray(`. FIR is correct to drop them: the
    // var is also assigned a runtime value, so the IV bytes need not be
    // literal.
    fun reassignedBefore(): IvParameterSpec {
        var prefix = "0123456789"
        prefix = loadPrefix()
        return IvParameterSpec("$prefix-abcde".toByteArray())
    }

    fun reassignedAfter(times: Int) {
        var prefix = "0123456789"
        repeat(times) {
            IvParameterSpec("$prefix-abcde".toByteArray())
        }
        prefix = loadPrefix()
    }

    fun reassignedInLambda(values: List<String>): IvParameterSpec {
        var prefix = "0123456789"
        values.forEach { prefix = it }
        return IvParameterSpec("$prefix-abcde".toByteArray())
    }

    fun reassignedInLocalFunction(): IvParameterSpec {
        var prefix = "0123456789"
        fun reset() {
            prefix = loadPrefix()
        }
        reset()
        return IvParameterSpec("$prefix-abcde".toByteArray())
    }

    fun compoundRuntimeOperand(): IvParameterSpec {
        var prefix = "0123456789"
        prefix += loadPrefix()
        return IvParameterSpec("$prefix-abcd".toByteArray())
    }

    // A literal reassignment does not make up for a runtime one.
    fun mixedReassignments(flag: Boolean): IvParameterSpec {
        var prefix = "0123456789"
        if (flag) prefix = "abcdefghij" else prefix = loadPrefix()
        return IvParameterSpec("$prefix-abcde".toByteArray())
    }
}
