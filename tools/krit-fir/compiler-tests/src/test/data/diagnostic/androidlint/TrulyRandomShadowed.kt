// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 16, 22, 27, 35, 44, 52, 61
// Divergence (precision): the file imports java.security.SecureRandom, but a
// local class, a nested class, and a parameter named SecureRandom shadow the
// import inside their scopes. Go reports every `SecureRandom(seed)` call in a
// file that imports java.security.SecureRandom; these calls do not construct
// java.security.SecureRandom, so FIR does not report them.
package test

import java.security.SecureRandom

fun jdk(): SecureRandom = <!TrulyRandom!>SecureRandom(byteArrayOf(1))<!>

fun local(): Any {
    class SecureRandom(val seed: ByteArray)
    return SecureRandom(byteArrayOf(1))
}

class Holder {
    class SecureRandom(val seed: ByteArray)

    fun nested(): Any = SecureRandom(byteArrayOf(1))
}

// A function-typed parameter named SecureRandom shadows the class: the call is
// that parameter's invoke, not a constructor call.
fun invoked(SecureRandom: (ByteArray) -> Any): Any = SecureRandom(byteArrayOf(1))

// Divergence (precision): a local function named SecureRandom shadows the
// import. Go reports the call because the file imports
// java.security.SecureRandom; the call is the local function, not a
// SecureRandom constructor, so FIR does not report it.
fun localFun(): Any {
    fun SecureRandom(seed: ByteArray): Any = seed
    return SecureRandom(byteArrayOf(9))
}

// Divergence (precision): a member function named SecureRandom wins over the
// imported class. Go reports the call; it is the member function, not a
// SecureRandom constructor, so FIR does not report it.
class Member {
    fun SecureRandom(seed: ByteArray): Any = seed

    fun use(): Any = SecureRandom(byteArrayOf(10))
}

// Divergence (precision): a local property holding a lambda shadows the
// import, and the call is that lambda's invoke. Go reports it; no SecureRandom
// is constructed, so FIR does not report it.
fun localLambda(): Any {
    val SecureRandom = { s: ByteArray -> s }
    return SecureRandom(byteArrayOf(11))
}

// Divergence (precision): inside with(x), K2 resolves the call to the String
// extension on the implicit receiver ahead of the explicit class import. Go
// reports it; the call is the extension, not a SecureRandom constructor, so FIR
// does not report it.
fun String.SecureRandom(seed: ByteArray): Int = seed.size

fun implicitReceiver(x: String): Any = with(x) { SecureRandom(byteArrayOf(14)) }
