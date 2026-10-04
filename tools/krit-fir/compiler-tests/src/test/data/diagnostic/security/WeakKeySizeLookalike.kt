// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Local lookalikes: a project KeyGenerator, a generator factory, and methods
// named init/initialize on other types never report.
package test.lookalike

class KeyGenerator {
    fun init(size: Int) {}

    companion object {
        fun getInstance(algorithm: String): KeyGenerator = KeyGenerator()
    }
}

class Generator {
    fun initialize(size: Int) {}
    fun init(size: Int) {}
}

object Factory {
    fun getInstance(algorithm: String): Generator = Generator()
}

class Lookalikes {
    fun keys(other: Generator) {
        other.initialize(1024)
        val rsa = Generator()
        rsa.initialize(1024)
        val fake = Factory.getInstance("RSA")
        fake.initialize(1024)
        fake.init(64)
        val local = KeyGenerator.getInstance("AES")
        local.init(64)
        val jdk = javax.crypto.KeyGenerator.getInstance("AES")
        jdk.init(128)
    }
}
