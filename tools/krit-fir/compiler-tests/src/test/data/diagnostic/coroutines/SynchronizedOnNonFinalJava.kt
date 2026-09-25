// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 34, 71
// Java-interop: a lock that is a non-final Java field, or a synthetic property
// over a Java getter/setter pair, is a non-final property just like a Kotlin
// `var`: it can be reassigned, which swaps the monitor object. FIR reports
// both, whether the Java class is a binary (the JDK) or a Java source stub
// (android.view.View). A getter with no setter is a `val` and is not
// reported.
package test

import android.content.Context
import android.view.View
import java.io.FilterOutputStream
import java.io.OutputStream
import java.io.Reader

// Go reports these because the class body also declares a local `var` of the
// same name; FIR agrees because the lock really is non-final: Reader.lock is a
// protected non-final field, and Thread.name has setName.
class MyReader : Reader() {
    override fun read(cbuf: CharArray, off: Int, len: Int): Int {
        <!SynchronizedOnNonFinal!>synchronized(lock) { return -1 }<!>
    }

    override fun close() {
        var lock = 0
        lock++
        println(lock)
    }
}

class MyThread : Thread() {
    fun f() {
        <!SynchronizedOnNonFinal!>synchronized(name) { }<!>
    }

    fun g() {
        var name = ""
        name += "x"
        println(name)
    }
}

// Go misses these because the class body declares no `var` of that name; FIR
// is correct because each lock is non-final: FilterOutputStream.out is a
// protected non-final field, Thread.contextClassLoader has a setter, and
// View.tag is the synthetic property over getTag/setTag.
class MyStream(out: OutputStream) : FilterOutputStream(out) {
    override fun flush() {
        <!SynchronizedOnNonFinal!>synchronized(out) { }<!>
    }
}

class LoaderThread : Thread() {
    fun f() {
        <!SynchronizedOnNonFinal!>synchronized(contextClassLoader) { }<!>
    }
}

class TaggedView(context: Context) : View(context) {
    fun use() {
        <!SynchronizedOnNonFinal!>synchronized(tag) { }<!>
    }
}

// Go reports this because g() declares a local `var threadGroup`; FIR is
// correct to drop it because Thread.threadGroup has no setter, so the lock is
// final.
class GroupThread : Thread() {
    fun f() {
        synchronized(threadGroup) { }
    }

    fun g() {
        var threadGroup = ""
        threadGroup += "x"
        println(threadGroup)
    }
}
