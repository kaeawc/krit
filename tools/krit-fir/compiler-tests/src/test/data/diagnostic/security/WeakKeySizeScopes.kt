// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: an init call chained on getInstance or made inside
// a scope function. Go takes only a bare variable receiver, and reads `it` as
// a variable named it, so it misses every finding below. FIR is correct
// because each receiver resolves to the generator the scope function was
// called on, created by getInstance with that algorithm, and the literal size
// is below the algorithm's minimum.
package test

import java.security.KeyPairGenerator
import javax.crypto.KeyGenerator

class Scopes(private val other: KeyGenerator) {
    fun chained() {
        <!WeakKeySize!>KeyPairGenerator.getInstance("RSA").initialize(1024)<!>
    }

    fun apply(): KeyPairGenerator = KeyPairGenerator.getInstance("RSA").apply { <!WeakKeySize!>initialize(1024)<!> }

    fun also(): KeyGenerator = KeyGenerator.getInstance("AES").also { <!WeakKeySize!>it.init(64)<!> }

    fun let() = KeyGenerator.getInstance("HmacSHA256").let { gen ->
        <!WeakKeySize!>gen.init(128)<!>
        gen.generateKey()
    }

    fun run() = KeyPairGenerator.getInstance("EC").run {
        <!WeakKeySize!>initialize(192)<!>
        generateKeyPair()
    }

    fun with() {
        with(KeyGenerator.getInstance("AES")) { <!WeakKeySize!>init(56)<!> }
    }

    fun safeCall(gen: KeyGenerator?) {
        gen?.apply { init(64) }
        KeyGenerator.getInstance("AES")?.apply { <!WeakKeySize!>init(64)<!> }
    }

    fun variableScope() {
        val gen = KeyPairGenerator.getInstance("DSA")
        gen.apply { <!WeakKeySize!>initialize(1024)<!> }
        with(gen) { <!WeakKeySize!>initialize(512)<!> }
    }

    fun nested() {
        KeyGenerator.getInstance("AES").also { outer ->
            outer.apply { <!WeakKeySize!>init(64)<!> }
        }
    }

    // Strong sizes, unknown algorithms, and generators that are not the
    // scope's receiver.
    fun negatives(name: String) {
        KeyPairGenerator.getInstance("RSA").initialize(2048)
        KeyGenerator.getInstance("AES").apply { init(256) }
        KeyGenerator.getInstance(name).apply { init(64) }
        KeyGenerator.getInstance("DES").also { it.init(56) }
        other.apply { init(64) }
        KeyGenerator.getInstance("AES").also { other.init(64) }
        listOf(other).forEach { it.init(64) }
    }
}
