// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): every call below either constructs java.util.Random or
// calls SecureRandom.setSeed with a fixed or time-based seed, and FIR reports
// it. Go misses each one: it proves a setSeed receiver only when the receiver
// is a SecureRandom constructor call or a name declared by a property whose
// type is spelled `SecureRandom` or whose initializer constructs one, and it
// recognizes only unsigned integer literals and `System.currentTimeMillis()` /
// `System.nanoTime()` spelled with exactly that two-part qualifier.
package test

import java.security.SecureRandom

typealias LegacyRandom = java.util.Random

class Holder(val rng: SecureRandom)

class SeededSubclass : SecureRandom() {
    init {
        // Implicit receiver: the class is a SecureRandom.
        <!SecureRandom!>setSeed(21L)<!>
    }
}

// A typealias of java.util.Random still constructs java.util.Random.
fun typeAlias(): java.util.Random = <!SecureRandom!>LegacyRandom()<!>

// A SecureRandom parameter.
fun parameter(rng: SecureRandom) {
    <!SecureRandom!>rng.setSeed(1L)<!>
}

// A getInstance result: with SHA1PRNG, a seed set first fixes the output.
fun getInstance() {
    val rng = SecureRandom.getInstance("SHA1PRNG")
    <!SecureRandom!>rng.setSeed(2L)<!>
    <!SecureRandom!>SecureRandom.getInstance("SHA1PRNG").setSeed(3L)<!>
}

// A member chain receiver.
fun memberChain(holder: Holder) {
    <!SecureRandom!>holder.rng.setSeed(4L)<!>
}

// A nullable property: its declared type is not spelled `SecureRandom`.
fun nullableProperty() {
    val rng: SecureRandom? = SecureRandom.getInstanceStrong()
    <!SecureRandom!>rng?.setSeed(5L)<!>
}

// A subclass constructor receiver.
fun subclassReceiver() {
    <!SecureRandom!>SeededSubclass().setSeed(6L)<!>
}

// A scope-function receiver.
fun scopeFunction(rng: SecureRandom) {
    with(rng) { <!SecureRandom!>setSeed(7L)<!> }
    rng.apply { <!SecureRandom!>setSeed(8L)<!> }
    rng.let { <!SecureRandom!>it.setSeed(9L)<!> }
}

// A negative literal is a fixed seed; tree-sitter parses it as a prefix
// expression, which Go does not count as a literal.
fun negativeLiteral(rng: SecureRandom) {
    <!SecureRandom!>rng.setSeed(-1L)<!>
}

// A fully qualified System call is still a time-based seed.
fun qualifiedSystem(rng: SecureRandom) {
    <!SecureRandom!>rng.setSeed(java.lang.System.nanoTime())<!>
}

// A smart cast from java.util.Random.
fun smartCast(r: java.util.Random) {
    if (r is SecureRandom) <!SecureRandom!>r.setSeed(10L)<!>
}

// A not-null assertion on the receiver.
fun notNullAssertion(rng: SecureRandom?) {
    <!SecureRandom!>rng!!.setSeed(11L)<!>
}

// A subclass that overrides setSeed: the resolved member is the override, and
// the class is still a SecureRandom.
class OverridingSubclass : SecureRandom() {
    override fun setSeed(seed: Long) {
        super.setSeed(seed)
    }
}

fun overridingSubclass() {
    <!SecureRandom!>OverridingSubclass().setSeed(12L)<!>
}

// A member of an object expression: the override's owner is the anonymous
// object, whose supertype is SecureRandom.
fun objectExpression() {
    val anonymous = object : SecureRandom() {
        override fun setSeed(seed: Long) {
            super.setSeed(seed)
        }

        fun reseedFixed() {
            <!SecureRandom!>setSeed(13L)<!>
        }
    }
    <!SecureRandom!>anonymous.setSeed(14L)<!>
}
