// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvement: Go misses these calls because the file declares a
// class named KeyGenerator and one named KeyPairGenerator (nested, so they
// shadow nothing here), and Go then treats every bare receiver with those
// names as a lookalike; FIR is correct because the receivers resolve to the
// imported JDK classes.
package test

import java.security.KeyPairGenerator
import javax.crypto.KeyGenerator

class Registry {
    class KeyGenerator
    class KeyPairGenerator
}

class DeclaredName {
    fun keys() {
        val aes = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>aes.init(64)<!>
        val rsa = KeyPairGenerator.getInstance("RSA")
        <!WeakKeySize!>rsa.initialize(1024)<!>
    }
}
