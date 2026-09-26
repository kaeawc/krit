// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 33, 42, 49, 59, 66, 80
// Deliberate precision fixes: Go reports every call below because it matches
// the receiver to a getInstance assignment by name and takes the first such
// getInstance in the function. FIR is correct because the generator the call
// initializes does not hold the algorithm Go reads: it is another variable
// with the same name, or the variable was reassigned on every path to the
// call with a generator whose minimum the size meets, or the value is another
// getInstance than the first one in it, or the receiver is not a crypto key
// generator at all, or the getInstance call is not the JDK's.
package test

import javax.crypto.KeyGenerator

class Precision {
    // The second `gen` is AES, not the earlier HmacSHA256 generator.
    fun earlierSibling() {
        run {
            val gen = KeyGenerator.getInstance("HmacSHA256")
            gen.init(256)
        }
        run {
            val gen = KeyGenerator.getInstance("AES")
            gen.init(128)
        }
    }

    // `gen` was reassigned to an AES generator before the call.
    fun reassigned() {
        var gen = KeyGenerator.getInstance("HmacSHA256")
        gen.init(256)
        gen = KeyGenerator.getInstance("AES")
        gen.init(128)
    }

    // Each destructuring entry takes its own Pair operand: `aes` is the AES
    // generator, not the HmacSHA256 one Go reads first, and 128 meets AES's
    // minimum.
    fun destructuring() {
        val (mac, aes) = Pair(KeyGenerator.getInstance("HmacSHA256"), KeyGenerator.getInstance("AES"))
        mac.init(256)
        aes.init(128)
    }

    // The same with `to`: `aes` is the AES generator.
    fun destructuringTo() {
        val (mac, aes) = KeyGenerator.getInstance("HmacSHA256") to KeyGenerator.getInstance("AES")
        mac.init(256)
        aes.init(128)
    }

    // The run block's value is its last statement, the AES generator; the
    // HmacSHA256 generator is only initialized inside it.
    fun runValue() {
        val gen: KeyGenerator = run {
            KeyGenerator.getInstance("HmacSHA256").init(256)
            KeyGenerator.getInstance("AES")
        }
        gen.init(128)
    }

    // `wrapped` is a Wrapper; the AES generator only appears in its
    // initializer's lambda.
    fun wrapper() {
        val wrapped = Wrapper().also { KeyGenerator.getInstance("AES") }
        wrapped.init(64)
    }
}

// A local `KeyGenerator` shadows the import, so `KeyGenerator.getInstance`
// below is the factory's, which returns an AES generator whatever the name;
// Go reads the name "HmacSHA256" as the algorithm.
object GeneratorFactory {
    fun getInstance(name: String): KeyGenerator = KeyGenerator.getInstance("AES")
}

fun shadowedReceiver() {
    val KeyGenerator = GeneratorFactory
    val gen = KeyGenerator.getInstance("HmacSHA256")
    gen.init(128)
}

class Wrapper {
    fun init(size: Int) {}
}
