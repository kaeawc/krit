// RENDER_DIAGNOSTICS_FULL_TEXT
// An IV template over a local var holds the var's literal initializer only
// when nothing in the function assigns the var again: a reassignment before
// the read, after it, or inside a lambda or local function can replace it.
package test

import javax.crypto.spec.IvParameterSpec

fun loadPrefix(): String = System.getenv("IV_PREFIX") ?: ""

class Crypto {
    // Never reassigned: Go reports it (a string literal followed by
    // `.toByteArray(`), and FIR matches it.
    fun neverReassigned(): IvParameterSpec {
        var prefix = "0123456789"
        return <!StaticIv!>IvParameterSpec("$prefix-abcde".toByteArray())<!>
    }

    // Go reports each of these because the argument starts with a string
    // literal followed by `.toByteArray(`. FIR is correct to drop them: the
    // var is reassigned in its function, so the IV bytes need not be the
    // literal. (Even a reassignment to another literal drops the finding:
    // only the initializer is trusted.)
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

    fun compoundAssignment(): IvParameterSpec {
        var prefix = "0123456789"
        prefix += "ab"
        return IvParameterSpec("$prefix-abcd".toByteArray())
    }
}
