// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 16
// Star imports: a bare KeyGenerator or KeyPairGenerator receiver resolves to
// the JDK class through `javax.crypto.*` or `java.security.*`, and both Go
// (its import facts count a star import of the package) and FIR report.
package test

import java.security.*
import javax.crypto.*

class StarImport {
    fun keys() {
        val aes = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>aes.init(64)<!>
        val rsa = KeyPairGenerator.getInstance("RSA")
        <!WeakKeySize!>rsa.initialize(1024)<!>
    }
}
