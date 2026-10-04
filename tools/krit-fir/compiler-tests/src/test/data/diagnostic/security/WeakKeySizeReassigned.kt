// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 32, 40, 47, 55, 62, 70, 78
// Reassignments that keep an earlier literal algorithm in play, and receivers
// whose own algorithm is unknown. FIR reports every call below, as Go does:
// a later write replaces the earlier ones only when it completes before the
// call, on every path, with a known algorithm on every path through its
// value; and a receiver whose own writes name no algorithm is read the way Go
// reads it.
package test

import java.security.KeyPairGenerator
import javax.crypto.KeyGenerator

const val RSA = "RSA"
const val HMAC = "HmacSHA256"
const val HMAC_ALIAS = HMAC

class Reassigned {
    // The reassignment's algorithm is a const val: `kpg` is RSA at the call.
    fun constReassigned() {
        var kpg = KeyPairGenerator.getInstance("RSA")
        kpg.initialize(2048)
        kpg = KeyPairGenerator.getInstance(RSA)
        <!WeakKeySize!>kpg.initialize(1024)<!>
    }

    // A const val first argument with a provider: `gen` is HmacSHA256.
    fun constReassignedProvider() {
        var gen = KeyGenerator.getInstance("HmacSHA256")
        gen.init(256)
        gen = KeyGenerator.getInstance(HMAC, "BC")
        <!WeakKeySize!>gen.init(128)<!>
    }

    // A const val that names another const val.
    fun constChain() {
        var gen = KeyGenerator.getInstance("HmacSHA256")
        gen.init(256)
        gen = KeyGenerator.getInstance(HMAC_ALIAS)
        <!WeakKeySize!>gen.init(128)<!>
    }

    // The call runs inside the reassignment's own value, before the
    // assignment: `gen` is still the HmacSHA256 generator.
    fun callInsideValue() {
        var gen = KeyGenerator.getInstance("HmacSHA256")
        gen = KeyGenerator.getInstance("AES").also { <!WeakKeySize!>gen.init(128)<!> }
        println(gen)
    }

    // When `cached` is null the generator is still the AES one.
    fun elvisKeepsGenerator(cached: KeyGenerator?) {
        var gen = KeyGenerator.getInstance("AES")
        gen = cached ?: gen
        <!WeakKeySize!>gen.init(64)<!>
    }

    // checkNotNull returns the same AES generator.
    fun checkNotNullKeepsGenerator() {
        var gen: KeyGenerator? = KeyGenerator.getInstance("AES")
        gen = checkNotNull(gen)
        <!WeakKeySize!>gen.init(64)<!>
    }

    // A reassignment whose algorithm is not known does not replace the AES
    // write; 64 bits is below every minimum the rule knows.
    fun reassignedDynamic(algorithm: String) {
        var gen = KeyGenerator.getInstance("AES")
        gen = KeyGenerator.getInstance(algorithm)
        <!WeakKeySize!>gen.init(64)<!>
    }

    // The lambda parameter's algorithm is unknown; Go reads the AES `gen`
    // above it by name, and 64 bits is below every minimum the rule knows.
    fun lambdaParameter(generators: List<KeyGenerator>) {
        val gen = KeyGenerator.getInstance("AES")
        gen.init(128)
        generators.forEach { gen -> <!WeakKeySize!>gen.init(64)<!> }
    }
}
