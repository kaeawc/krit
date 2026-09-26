// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 17, 18, 20, 22, 24, 26, 28, 30, 32, 34, 35, 38, 40, 42, 44, 47, 50, 53, 57, 67, 119
// Positives and negatives for WeakKeySize: KeyGenerator.init and
// KeyPairGenerator.initialize with a literal size below the minimum for the
// algorithm the generator variable was created with.
package test

import java.security.KeyPairGenerator
import java.security.SecureRandom
import javax.crypto.KeyGenerator

class Crypto {
    fun weak(random: SecureRandom) {
        val rsa = KeyPairGenerator.getInstance("RSA")
        <!WeakKeySize!>rsa.initialize(1024)<!>
        val aes = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>aes.init(64)<!>
        <!WeakKeySize!>aes.init(64, random)<!>
        val dsa = KeyPairGenerator.getInstance("DSA", "SUN")
        <!WeakKeySize!>dsa.initialize(1024, random)<!>
        val ec = KeyPairGenerator.getInstance("EC")
        <!WeakKeySize!>ec.initialize(192)<!>
        val ecdsa = KeyPairGenerator.getInstance("ecdsa")
        <!WeakKeySize!>ecdsa.initialize(160)<!>
        val hmac = KeyGenerator.getInstance("HmacSHA256")
        <!WeakKeySize!>hmac.init(128)<!>
        val dashed = KeyGenerator.getInstance(" hmac-sha512 ")
        <!WeakKeySize!>dashed.init(255)<!>
        val raw = KeyPairGenerator.getInstance("""RSA""")
        <!WeakKeySize!>raw.initialize(1_024)<!>
        val paren = javax.crypto.KeyGenerator.getInstance(("AES"))
        <!WeakKeySize!>paren.init((96))<!>
        val qualified = java.security.KeyPairGenerator.getInstance("RSA")
        <!WeakKeySize!>qualified.initialize(-1)<!>
        <!WeakKeySize!>qualified<!>
            .initialize(512)
        val typed: KeyPairGenerator = KeyPairGenerator.getInstance("RSA")
        <!WeakKeySize!>typed.initialize(+1024)<!>
        val nullable: KeyGenerator? = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>nullable?.init(64)<!>
        val any: Any = KeyGenerator.getInstance("AES")
        if (any is KeyGenerator) <!WeakKeySize!>any.init(64)<!>
        val chained = KeyGenerator.getInstance("AES").also { println(it) }
        <!WeakKeySize!>chained.init(64)<!>
        var assigned: KeyPairGenerator
        assigned = KeyPairGenerator.getInstance("RSA")
        <!WeakKeySize!>assigned.initialize(1024)<!>
        val lambda = {
            val inner = KeyGenerator.getInstance("AES")
            <!WeakKeySize!>inner.init(64)<!>
        }
        val outer = KeyGenerator.getInstance("AES")
        run { <!WeakKeySize!>outer.init(64)<!> }
        val inObject = object {
            fun weak() {
                val gen = KeyGenerator.getInstance("AES")
                <!WeakKeySize!>gen.init(64)<!>
            }
        }
        inObject.weak()
    }

    // A destructuring declaration gives every entry the first getInstance
    // algorithm in its value, as Go reads it: `second` is taken as AES.
    fun destructuring() {
        val (first, second) = Pair(KeyGenerator.getInstance("AES"), KeyPairGenerator.getInstance("RSA"))
        <!WeakKeySize!>first.init(64)<!>
        second.initialize(1024)
    }

    fun strong(size: Int) {
        val rsa = KeyPairGenerator.getInstance("RSA")
        rsa.initialize(2048)
        rsa.initialize(4096)
        rsa.initialize(size)
        rsa.initialize(0x400)
        rsa.initialize(KEY_BITS)
        rsa.initialize(- 512)
        val aes = KeyGenerator.getInstance("AES")
        aes.init(128)
        aes.init(256)
        aes.init(SecureRandom())
        val ec = KeyPairGenerator.getInstance("EC")
        ec.initialize(256)
        ec.initialize(224)
        val hmac = KeyGenerator.getInstance("HmacSHA256")
        hmac.init(256)
        val unknown = KeyGenerator.getInstance("DESede")
        unknown.init(56)
        val dynamic = KeyGenerator.getInstance(algorithm())
        dynamic.init(8)
        val template = KeyGenerator.getInstance("${"AES"}")
        template.init(8)
        val escaped = KeyPairGenerator.getInstance("R\u0053A")
        escaped.initialize(512)
    }

    // The getInstance call comes after the init call.
    fun later() {
        lateinit var gen: KeyGenerator
        val init = { gen.init(64) }
        gen = KeyGenerator.getInstance("AES")
        init()
    }

    private fun algorithm(): String = "AES"
}

const val KEY_BITS = 1024

// An inherited member property assigned in the function.
abstract class Base {
    lateinit var gen: KeyGenerator
}

class Sub : Base() {
    fun weak() {
        gen = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>gen.init(64)<!>
    }
}
