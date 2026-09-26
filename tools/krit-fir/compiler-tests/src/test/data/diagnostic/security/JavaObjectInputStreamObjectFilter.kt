// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 54, 62
// A raw ObjectInputStream built inside a filtering subclass written as an
// object (an object literal, a named object, or a companion object). FIR skips
// a call only when its nearest enclosing class (objects looked through, like
// Go's class_declaration walk) is a filtering subclass, so every call below is
// reported: each one builds an unfiltered java.io.ObjectInputStream.
//
// Go skips the calls whose object sits inside a class, because that class's
// text mentions ObjectInputStream and resolveClass (the object's own body
// does). Exempting object filters in FIR would match those, but would lose
// Go's true positive for the top-level object filter, where Go has no
// enclosing class_declaration and reports. FIR keeps the finding everywhere.
package test

import java.io.ByteArrayInputStream
import java.io.InputStream
import java.io.ObjectInputStream
import java.io.ObjectStreamClass

class Decoder {
    // Go misses this because Decoder's text mentions resolveClass (in the
    // object literal); FIR is correct because the call builds a raw stream
    // and Decoder is not a filtering subclass.
    fun safe(input: InputStream): Any = object : ObjectInputStream(input) {
        override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)

        fun raw(i: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(i)<!>
    }.readObject()

    // Go misses this for the same reason (the nested object's text is part of
    // Decoder's text); FIR reports the raw stream.
    object NamedFilter : ObjectInputStream(ByteArrayInputStream(ByteArray(0))) {
        override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)

        fun raw(i: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(i)<!>
    }
}

class Holder {
    // Go misses this for the same reason (the companion's text is part of
    // Holder's text); FIR reports the raw stream.
    companion object Filter : java.io.ObjectInputStream(ByteArrayInputStream(ByteArray(0))) {
        override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)

        fun raw(i: InputStream) = <!JavaObjectInputStream!>java.io.ObjectInputStream(i)<!>
    }
}

// Both report: a top-level object has no enclosing class_declaration in Go.
object TopFilter : ObjectInputStream(ByteArrayInputStream(ByteArray(0))) {
    override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)

    fun raw(i: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(i)<!>
}

// Both report: an object literal in a top-level function has no enclosing
// class_declaration in Go either.
fun topLevelLiteral(input: InputStream): Any = object : ObjectInputStream(input) {
    override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)

    fun raw(i: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(i)<!>
}.readObject()

// Both skip: a class filtering subclass is still exempt when the call sits in
// an object nested in it, because the object is looked through.
class ClassFilter(input: InputStream) : ObjectInputStream(input) {
    override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)

    companion object {
        fun raw(i: InputStream): ObjectInputStream = ObjectInputStream(i)
    }
}
