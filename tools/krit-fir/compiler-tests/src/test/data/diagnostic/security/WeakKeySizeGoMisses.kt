// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: Go misses every finding below. Go reads the
// generator's algorithm only from a getInstance call whose receiver is spelled
// `KeyGenerator`, `KeyPairGenerator`, or their qualified names, inside the
// init call's nearest enclosing function_declaration, assigned to a variable
// whose name matches the receiver text, taking the first such assignment in
// the function. FIR is correct because in each case the receiver resolves to
// a generator created by javax.crypto.KeyGenerator or
// java.security.KeyPairGenerator.getInstance with that algorithm, and the
// literal size is below the algorithm's minimum.
package test

import java.security.KeyPairGenerator
import javax.crypto.KeyGenerator
import javax.crypto.KeyGenerator as JKeyGen

typealias Kpg = KeyPairGenerator

val topLevel: KeyPairGenerator = KeyPairGenerator.getInstance("RSA")

fun topLevelVal() {
    // Go misses: the val is initialized outside the function.
    <!WeakKeySize!>topLevel.initialize(1024)<!>
}

class GoMisses {
    private val member = KeyGenerator.getInstance("AES")
    lateinit var late: KeyGenerator

    // Go misses: no function_declaration encloses an init block.
    init {
        val gen = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>gen.init(64)<!>
        late = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>late.init(64)<!>
    }

    // Go misses: no function_declaration encloses a property initializer.
    val initialized = run {
        val gen = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>gen.init(64)<!>
        gen
    }

    // Go misses: the val is initialized outside the function.
    fun memberVal() {
        <!WeakKeySize!>member.init(64)<!>
        val inObject = object {
            val gen = KeyGenerator.getInstance("AES")
            fun weak() {
                <!WeakKeySize!>gen.init(64)<!>
            }
        }
        inObject.weak()
    }

    // Go misses: the getInstance receiver is an import alias, a typealias, or
    // parenthesized, so Go does not recognize it.
    fun spellings() {
        val aliased = JKeyGen.getInstance("AES")
        <!WeakKeySize!>aliased.init(64)<!>
        val viaTypealias = Kpg.getInstance("RSA")
        <!WeakKeySize!>viaTypealias.initialize(1024)<!>
        val parenthesized = (KeyGenerator).getInstance("AES")
        <!WeakKeySize!>parenthesized.init(64)<!>
    }

    // Go misses: the variable is declared outside the init call's nearest
    // function_declaration.
    fun localFunction() {
        val gen = KeyGenerator.getInstance("AES")
        fun weak() {
            <!WeakKeySize!>gen.init(64)<!>
        }
        weak()
    }

    fun localClass() {
        val gen = KeyPairGenerator.getInstance("RSA")
        class Holder {
            fun weak() {
                <!WeakKeySize!>gen.initialize(1024)<!>
            }
        }
        Holder().weak()
    }

    // Go misses: Go reads only the first assignment (AES, where 128 is
    // enough); the conditional HmacSHA256 assignment also reaches the call.
    fun conditional(strong: Boolean) {
        var gen = KeyGenerator.getInstance("AES")
        if (!strong) gen = KeyGenerator.getInstance("HmacSHA256")
        <!WeakKeySize!>gen.init(128)<!>
    }

    // Go misses: Go reads the first variable named `gen` in the function
    // (AES) for both calls.
    fun reusedName() {
        run {
            val gen = KeyGenerator.getInstance("AES")
            gen.init(128)
        }
        run {
            val gen = KeyPairGenerator.getInstance("RSA")
            <!WeakKeySize!>gen.initialize(1024)<!>
        }
    }

    // Go misses: Go reads the annotation, label, or comment node as the
    // argument, or compares the receiver text `(gen)` or `` `gen` `` with
    // `gen`.
    fun arguments() {
        val gen = KeyGenerator.getInstance("AES")
        <!WeakKeySize!>gen.init(@Suppress("MagicNumber") 64)<!>
        <!WeakKeySize!>gen.init(bits@ 64)<!>
        <!WeakKeySize!>gen.init((/* bits */ 64))<!>
        <!WeakKeySize!>(gen).init(64)<!>
        <!WeakKeySize!>`gen`.init(64)<!>
    }

    // Go misses: a when subject variable is not a declaration Go reads.
    fun whenSubject() {
        when (val gen = KeyGenerator.getInstance("AES")) {
            else -> <!WeakKeySize!>gen.init(64)<!>
        }
    }

    // Go misses: Go searches only the anonymous object's `run` method; the
    // local is the AES generator.
    fun anonymousObjectMethod() {
        val gen = KeyGenerator.getInstance("AES")
        val task = object : Runnable {
            override fun run() {
                <!WeakKeySize!>gen.init(64)<!>
            }
        }
        task.run()
    }

    // Go misses: no function_declaration encloses a property getter.
    val bits: Int
        get() {
            val gen = KeyGenerator.getInstance("AES")
            <!WeakKeySize!>gen.init(64)<!>
            return 64
        }

    // Go misses: Go reads only a string literal algorithm; RSA_NAME is the
    // const val "RSA".
    fun constAlgorithm() {
        val kpg = KeyPairGenerator.getInstance(RSA_NAME)
        <!WeakKeySize!>kpg.initialize(1024)<!>
    }
}

const val RSA_NAME = "RSA"

// Go misses: no function_declaration encloses a constructor parameter's
// default value.
class ConstructorDefault(val x: Int = run {
    val gen = KeyGenerator.getInstance("AES")
    <!WeakKeySize!>gen.init(64)<!>
    1
})
