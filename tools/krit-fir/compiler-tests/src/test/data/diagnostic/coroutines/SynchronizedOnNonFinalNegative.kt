// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: locks that are not a bare name resolving to a `var` must not
// trigger SynchronizedOnNonFinal. Go agrees on every case in this file.
package test

import android.content.Context
import android.view.View

val topLevelLock = Any()

class Service(val ctorLock: Any) {
    private val lock = Any()
    private var mutable = Any()
    val computed: Any
        get() = Any()

    fun finalLocks(param: Any) {
        synchronized(lock) { }
        synchronized(this) { }
        synchronized(param) { }
        synchronized(ctorLock) { }
        synchronized(topLevelLock) { }
        synchronized(computed) { }
        synchronized(Service::class.java) { }
        synchronized(Companion) { }
        val local = Any()
        synchronized(local) { }
        synchronized(listOf(1)) { }
        synchronized("text") { }
    }

    // Go only inspects a bare name as the first unlabelled argument.
    fun notABareName(other: Service) {
        synchronized(this.mutable) { }
        synchronized(other.mutable) { }
        synchronized((mutable)) { }
        synchronized(mutable as Any) { }
        synchronized(lock = mutable) { }
        synchronized(block = { }, lock = mutable)
        synchronized(mutable.also { }) { }
    }

    // Not a call written `synchronized`.
    fun otherCalls() {
        lockOn(mutable) { }
        mutable.hashCode()
        kotlin.run { mutable.toString() }
    }

    private fun <R> lockOn(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)

    // A lambda parameter is final.
    fun lambdaParam() {
        listOf(Any()).forEach { item -> synchronized(item) { } }
        listOf(Any()).forEach { synchronized(it) { } }
    }

    // A loop variable and a catch parameter are final.
    fun loopAndCatch(items: List<Any>) {
        for (item in items) {
            synchronized(item) { }
        }
        try {
            println(items)
        } catch (e: Exception) {
            synchronized(e) { }
        }
    }

    companion object
}

// Java-interop: a synthetic property with no setter is a `val`
// (View.getContext has no setContext), and Go sees no `var context` either.
class ContextView(context: Context) : View(context) {
    fun use() {
        synchronized(context) { }
    }
}

fun topLevel() {
    val finalLock = Any()
    synchronized(finalLock) { }
}
