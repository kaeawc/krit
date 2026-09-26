// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go skips every call whose nearest enclosing class mentions both
// ObjectInputStream and resolveClass. FIR skips only a real filtering
// subclass: one that extends java.io.ObjectInputStream and declares
// resolveClass. More text-mention shapes are in
// JavaObjectInputStreamTextMention.kt and JavaObjectInputStreamObjectFilter.kt.
package test

import java.io.InputStream
import java.io.ObjectInputStream
import java.io.ObjectStreamClass

open class BaseFilter(input: InputStream) : ObjectInputStream(input) {
    override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)
}

// An indirect subclass that declares resolveClass is still a filtering
// subclass, so the call is skipped like Go.
class DerivedFilter(input: InputStream) : BaseFilter(input) {
    override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)

    fun raw(input: InputStream): ObjectInputStream = ObjectInputStream(input)
}

class Outer {
    private class Filtering(input: InputStream) : ObjectInputStream(input) {
        override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)

        // The nearest class is Filtering: skipped like Go.
        fun raw(input: InputStream): ObjectInputStream = ObjectInputStream(input)
    }

    fun safe(input: InputStream): ObjectInputStream = Filtering(input)

    // Go misses this because Outer's text mentions resolveClass (in the nested
    // Filtering class); FIR is correct because Outer is not a filtering
    // subclass and this call builds a raw ObjectInputStream.
    fun raw(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
}

class Commented {
    // Go misses this because this comment says resolveClass; FIR is correct
    // because Commented is not a filtering subclass.
    fun raw(input: InputStream): ObjectInputStream = <!JavaObjectInputStream!>ObjectInputStream(input)<!>
}

fun localFilter(input: InputStream): Any {
    // A local filtering subclass counts like a top-level one.
    class LocalFilter(input: InputStream) : ObjectInputStream(input) {
        override fun resolveClass(desc: ObjectStreamClass): Class<*> = super.resolveClass(desc)

        fun raw(): ObjectInputStream = ObjectInputStream(input)
    }
    return LocalFilter(input).raw()
}
