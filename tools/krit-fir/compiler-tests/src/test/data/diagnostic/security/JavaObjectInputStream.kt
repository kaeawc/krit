// RENDER_DIAGNOSTICS_FULL_TEXT
// Positives and negatives for JavaObjectInputStream: a constructor call that
// creates a java.io.ObjectInputStream, reported on the call like Go.
package test

import java.io.ByteArrayInputStream
import java.io.FileInputStream
import java.io.InputStream
import java.io.ObjectInputStream
import java.io.ObjectStreamClass

val topLevel = <!JavaObjectInputStream!>ObjectInputStream(ByteArrayInputStream(ByteArray(0)))<!>

fun topLevelFunction(input: InputStream): Any = <!JavaObjectInputStream!>ObjectInputStream(input)<!>.readObject()

class Decoder {
    val field: ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(ByteArrayInputStream(ByteArray(0)))<!>

    fun decode(path: String): Any {
        return <!JavaObjectInputStream!>ObjectInputStream(FileInputStream(path))<!>.use { it.readObject() }
    }

    fun qualified(input: InputStream): Any = <!JavaObjectInputStream!>java.io.ObjectInputStream(input)<!>.readObject()

    fun qualifiedMultiline(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>java.io<!>
        .ObjectInputStream(input)

    fun multiline(input: InputStream): ObjectInputStream =
        <!JavaObjectInputStream!>ObjectInputStream(<!>
            input,
        )

    fun inLambda(input: InputStream): () -> ObjectInputStream = { <!JavaObjectInputStream!>ObjectInputStream(input)<!> }

    fun nested(input: InputStream): ObjectInputStream =
        <!JavaObjectInputStream!>ObjectInputStream(<!>
            <!JavaObjectInputStream!>ObjectInputStream(input)<!>,
        )

    fun localClass(input: InputStream): Any {
        class Local {
            fun open(): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
        }
        return Local().open()
    }

    fun anonymousObjectMember(input: InputStream): Any {
        val holder = object {
            fun open(): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
        }
        return holder.open()
    }

    companion object {
        fun open(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
    }
}

object Singleton {
    fun open(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
}

interface Opener {
    fun open(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
}

enum class Mode {
    RAW {
        override fun open(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
    };

    abstract fun open(input: InputStream): ObjectInputStream
}

// A filtering subclass: its supertype delegation is not a constructor call,
// and calls inside it (including its companion) are skipped like Go.
class FilteringInputStream(input: InputStream) : ObjectInputStream(input) {
    override fun resolveClass(desc: ObjectStreamClass): Class<*> {
        return super.resolveClass(desc)
    }

    fun reopen(input: InputStream): ObjectInputStream = ObjectInputStream(input)

    val lazyStream: () -> ObjectInputStream = { ObjectInputStream(ByteArrayInputStream(ByteArray(0))) }

    companion object {
        fun raw(input: InputStream): ObjectInputStream = ObjectInputStream(input)
    }
}

// An ObjectInputStream subclass that does not override resolveClass is not a
// filtering subclass, so a raw stream built in it is reported like Go.
class PlainSubclass(input: InputStream) : ObjectInputStream(input) {
    fun reopen(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
}

class Negatives {
    fun subclass(input: InputStream): ObjectInputStream = FilteringInputStream(input)

    fun anonymousFilter(input: InputStream): ObjectInputStream = object : ObjectInputStream(input) {
        override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)
    }

    fun reference(): (InputStream) -> ObjectInputStream = ::ObjectInputStream

    fun mention() {
        println("never call java.io.ObjectInputStream(input) directly")
    }
}
