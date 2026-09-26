// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 31, 40, 47, 54, 68
// Deliberate precision fixes: Go reports every call below because it matches
// the receiver to a getInstance assignment by name and takes the first such
// assignment in the function. FIR is correct because the generator the call
// initializes does not hold the algorithm Go reads: it is another variable
// with the same name, or the variable was reassigned, on every path to the
// call, with a generator whose minimum the size meets or whose algorithm is
// not a literal, or the receiver is not a crypto key generator at all, or the
// getInstance call is not the JDK's.
package test

import javax.crypto.KeyGenerator

class Precision {
    // The lambda parameter `gen` is not the AES generator.
    fun lambdaParameter(generators: List<KeyGenerator>) {
        val gen = KeyGenerator.getInstance("AES")
        gen.init(128)
        generators.forEach { gen -> gen.init(64) }
    }

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

    // `gen` was reassigned to a generator whose algorithm is not a literal.
    fun reassignedDynamic(algorithm: String) {
        var gen = KeyGenerator.getInstance("AES")
        gen = KeyGenerator.getInstance(algorithm)
        gen.init(64)
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
