// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: an algorithm named by a final Java constant field,
// the usual Android form. Go reads only a string literal algorithm, so it
// misses each finding below. FIR is correct because the field's constant value
// names the algorithm, and the literal size is below its minimum.
package test

import android.security.keystore.KeyProperties
import java.security.KeyPairGenerator
import javax.crypto.KeyGenerator

class JavaConstants {
    fun rsa() {
        val generator = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_RSA)
        <!WeakKeySize!>generator.initialize(1024)<!>
    }

    fun ec() {
        val generator = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_EC, "AndroidKeyStore")
        <!WeakKeySize!>generator.initialize(192)<!>
    }

    fun aes() = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES).apply { <!WeakKeySize!>init(64)<!> }

    fun hmac() {
        val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_HMAC_SHA256)
        <!WeakKeySize!>generator.init(128)<!>
    }

    fun strong() {
        val generator = KeyPairGenerator.getInstance(KeyProperties.KEY_ALGORITHM_RSA)
        generator.initialize(2048)
        KeyGenerator.getInstance(KeyProperties.BLOCK_MODE_GCM).init(64)
    }
}
