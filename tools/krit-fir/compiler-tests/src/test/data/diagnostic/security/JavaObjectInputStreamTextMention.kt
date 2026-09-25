// RENDER_DIAGNOSTICS_FULL_TEXT
// Go's safe-scope check is a text match: it skips a call when the text of the
// nearest class_declaration contains ObjectInputStream and resolveClass. The
// call itself always supplies the ObjectInputStream mention, so Go skips any
// raw call in a class whose text mentions resolveClass anywhere. FIR skips only
// a real filtering subclass (extends java.io.ObjectInputStream and declares
// resolveClass), so every call below is reported: each builds an unfiltered
// java.io.ObjectInputStream, and Go misses it.
package test

import java.io.InputStream
import java.io.ObjectInputStream
import java.io.ObjectStreamClass

// Go misses this because the interface declares a function named
// resolveClass; an interface is not a filtering subclass.
interface SafeOpener {
    fun resolveClass(name: String): Class<*>

    fun open(i: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(i)<!>
}

// Go misses this because Loader declares a resolveClass helper; Loader does
// not extend ObjectInputStream, so the stream it builds is unfiltered.
class Loader(private val loader: ClassLoader) {
    fun resolveClass(desc: ObjectStreamClass): Class<*> = Class.forName(desc.name, false, loader)

    fun open(i: InputStream): java.io.ObjectInputStream = <!JavaObjectInputStream!>java.io.ObjectInputStream(i)<!>
}

// Go misses this because a string literal in the class says resolveClass.
class Messages {
    val hint = "override resolveClass to allowlist"

    fun open(i: InputStream): java.io.ObjectInputStream = <!JavaObjectInputStream!>java.io.ObjectInputStream(i)<!>
}

// Go misses this because entry A's body is an anonymous filtering subclass,
// which puts resolveClass in the enum class's text; entry B builds a raw
// stream, and Kind is not a filtering subclass.
enum class Kind {
    A {
        override fun open(i: InputStream): java.io.ObjectInputStream = object : java.io.ObjectInputStream(i) {
            override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)
        }
    },
    B {
        override fun open(i: InputStream): java.io.ObjectInputStream = <!JavaObjectInputStream!>java.io.ObjectInputStream(i)<!>
    };

    abstract fun open(i: InputStream): java.io.ObjectInputStream
}
