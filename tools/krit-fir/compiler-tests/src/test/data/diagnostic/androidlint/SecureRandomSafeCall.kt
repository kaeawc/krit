// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 19, 33
// A java.util.Random-typed variable whose initializer constructs a
// SecureRandom holds a SecureRandom, so a literal seed through a safe call on
// it is a fixed seed on a SecureRandom. K2 does not smart-cast the variable
// from its initializer, and the safe call's receiver is a checked safe-call
// subject, so the checker has to look through it to the variable.
package test

import java.security.SecureRandom
import java.util.Random

class SafeCallHolder {
    private val upcastNullable: Random? = SecureRandom()
    private val upcastNonNull: Random = SecureRandom()

    fun seed() {
        <!SecureRandom!>upcastNullable?.setSeed(1L)<!>
        <!SecureRandom!>upcastNonNull?.setSeed(2L)<!>
    }

    // Divergence (recall): a not-null assertion on the same variable. Go
    // proves only a bare name, a constructor call, or a java.security
    // qualifier as the receiver, so it misses `x!!`; the receiver still holds
    // a SecureRandom seeded with a literal.
    fun asserted() {
        <!SecureRandom!>upcastNullable!!.setSeed(4L)<!>
    }
}

fun localSafeCall() {
    val localNullable: Random? = SecureRandom()
    <!SecureRandom!>localNullable?.setSeed(3L)<!>
}
