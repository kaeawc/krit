// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 14, 16, 18, 20, 22, 24, 26, 29, 36, 43, 50, 54
// Positives for HardcodedRandomSeed: a java.security.SecureRandom constructor call that
// passes a seed. Any seed argument counts, as in the Go rule.
package test

import java.security.SecureRandom

class Seeds {
    fun literal(): SecureRandom = <!HardcodedRandomSeed!>SecureRandom(byteArrayOf(1, 2, 3))<!>

    fun parameter(seed: ByteArray): SecureRandom = <!HardcodedRandomSeed!>SecureRandom(seed)<!>

    fun generated(): SecureRandom = <!HardcodedRandomSeed!>SecureRandom(SecureRandom().generateSeed(20))<!>

    fun qualified(): SecureRandom = <!HardcodedRandomSeed!>java.security.SecureRandom(byteArrayOf(1))<!>

    fun chained(): Int = <!HardcodedRandomSeed!>SecureRandom(byteArrayOf(1))<!>.nextInt(5)

    fun qualifiedChained(): Int = <!HardcodedRandomSeed!>java.security.SecureRandom(byteArrayOf(1))<!>.nextInt(5)

    fun parenthesized(): SecureRandom = <!HardcodedRandomSeed!>SecureRandom((byteArrayOf(1)))<!>

    fun spread(seeds: Array<Byte>): SecureRandom = <!HardcodedRandomSeed!>SecureRandom(seeds.toByteArray())<!>

    fun inLambda(): SecureRandom = run { <!HardcodedRandomSeed!>SecureRandom(byteArrayOf(1))<!> }

    fun inAnonymousObject(): Any = object {
        val random = <!HardcodedRandomSeed!>SecureRandom(byteArrayOf(1))<!>
    }

    // Go reports on the first line of the call expression: the package
    // qualifier of a split qualified call, and the callee of a call whose
    // arguments wrap.
    fun splitQualified(): SecureRandom {
        val random = <!HardcodedRandomSeed!>java.security<!>
            .SecureRandom(byteArrayOf(1))
        return random
    }

    fun wrappedArguments(): SecureRandom {
        val random =
            <!HardcodedRandomSeed!>SecureRandom(<!>
                byteArrayOf(1),
            )
        return random
    }

    companion object {
        val SHARED = <!HardcodedRandomSeed!>SecureRandom(byteArrayOf(9))<!>
    }
}

val topLevel = <!HardcodedRandomSeed!>SecureRandom(byteArrayOf(1))<!>
