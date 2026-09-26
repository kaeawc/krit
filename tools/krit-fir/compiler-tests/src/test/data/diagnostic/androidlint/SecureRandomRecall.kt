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

// A lazy delegate: the property's type is inferred from the lazy
// initializer. Go misses it: the declaration has no SecureRandom type and no
// constructor call of its own.
fun lazyDelegate() {
    val lazyRng by lazy { SecureRandom() }
    <!SecureRandom!>lazyRng.setSeed(8L)<!>
}

// A java.util.Random-typed variable initialized through a typealias of
// SecureRandom, or with a local SecureRandom subclass: it holds a SecureRandom.
// Go misses both: the constructor is not spelled `SecureRandom`.
typealias SR = SecureRandom

fun aliasInitializer() {
    val viaAlias: java.util.Random = SR()
    <!SecureRandom!>viaAlias.setSeed(15L)<!>
}

fun localSubclassInitializer() {
    class LocalSecure : SecureRandom()
    val viaLocal: java.util.Random = LocalSecure()
    <!SecureRandom!>viaLocal.setSeed(16L)<!>
}

// A member of an object expression whose initializer constructs a
// SecureRandom. Go misses it: it does not prove a member chain receiver.
fun anonymousMember() {
    val anon = object {
        val inner: java.util.Random = SecureRandom()
    }
    <!SecureRandom!>anon.inner.setSeed(17L)<!>
}
