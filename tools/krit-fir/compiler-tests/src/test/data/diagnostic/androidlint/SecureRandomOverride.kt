// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// The finding names SecureRandom.setSeed(), so only a call that resolves to
// the JDK's own setSeed counts: java.security.SecureRandom.setSeed, or
// java.util.Random.setSeed on a variable that holds a SecureRandom. A project
// overload or override of setSeed is not known to seed the generator.
package test

import java.security.SecureRandom

// An overload on a subclass: `setSeed(6)` resolves to Overload.setSeed(Int),
// which only prints its argument, so nothing is seeded. Go does not report it
// either (the receiver is not spelled SecureRandom).
class Overload : SecureRandom() {
    fun setSeed(seed: Int) {
        println(seed)
    }
}

fun overload() {
    Overload().setSeed(6)
}

// A no-op override: the call does not seed anything, so the message would be
// false. Go does not report it either.
class Noop : SecureRandom() {
    override fun setSeed(seed: Long) {}
}

fun noop() {
    Noop().setSeed(7L)
}

// An override that delegates to super does seed the generator, but the checker
// does not read override bodies, so it treats every project override like the
// no-op one above. Go does not report these calls either.
class OverridingSubclass : SecureRandom() {
    override fun setSeed(seed: Long) {
        super.setSeed(seed)
    }
}

fun overridingSubclass() {
    OverridingSubclass().setSeed(12L)
}

fun objectExpression() {
    val anonymous = object : SecureRandom() {
        override fun setSeed(seed: Long) {
            super.setSeed(seed)
        }

        fun reseedFixed() {
            setSeed(13L)
        }
    }
    anonymous.setSeed(14L)
}

// Divergence (recall): `super.setSeed` and `this.setSeed` in a SecureRandom
// subclass resolve to SecureRandom.setSeed itself, a literal seed on a
// SecureRandom. Go misses both: `super` and `this` are neither a constructor
// call nor a name declared by a SecureRandom property.
class SuperSeeded : SecureRandom() {
    override fun setSeed(seed: Long) {
        <!SecureRandom!>super.setSeed(9L)<!>
    }
}

class ThisSeeded : SecureRandom() {
    fun seedFixed() {
        <!SecureRandom!>this.setSeed(10L)<!>
    }
}
