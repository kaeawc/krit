// RENDER_DIAGNOSTICS_FULL_TEXT
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
