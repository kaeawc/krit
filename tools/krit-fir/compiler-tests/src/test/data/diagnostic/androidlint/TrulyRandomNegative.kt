// RENDER_DIAGNOSTICS_FULL_TEXT
// Negatives for TrulyRandom: the default constructor, factory methods, setSeed
// (SecureRandom's rule, not this one), a subclass constructor, superclass
// delegation, and a constructor reference. Go reports none of these either:
// superclass delegation and `::SecureRandom` are not constructor call
// expressions.
package test

import java.security.SecureRandom

class SeededRandom(seed: ByteArray) : SecureRandom(seed)

class DelegatingRandom : SecureRandom {
    constructor(seed: ByteArray) : super(seed)
}

class Randoms {
    fun defaults(): SecureRandom = SecureRandom()

    fun strong(): SecureRandom = SecureRandom.getInstanceStrong()

    fun named(): SecureRandom = SecureRandom.getInstance("SHA1PRNG")

    fun reseeded(random: SecureRandom) {
        random.setSeed(byteArrayOf(1))
        random.setSeed(42L)
    }

    fun subclass(): SecureRandom = SeededRandom(byteArrayOf(1))

    fun delegating(): SecureRandom = DelegatingRandom(byteArrayOf(1))

    fun anonymousSubclass(): SecureRandom = object : SecureRandom(byteArrayOf(1)) {}

    fun reference(seeds: List<ByteArray>): List<SecureRandom> = seeds.map(::SecureRandom)

    fun otherRandoms(): Any = listOf(java.util.Random(42L), kotlin.random.Random(42))
}
