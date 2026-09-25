// RENDER_DIAGNOSTICS_FULL_TEXT
// Positive: synchronized() whose lock is a boxed primitive literal or a
// property declared with a primitive type in the enclosing class/object
// should trigger SynchronizedOnBoxedPrimitive.
//
// From language version 2.1, K2 rejects kotlin.synchronized on a primitive
// with the error SYNCHRONIZED_BLOCK_ON_VALUE_CLASS_OR_PRIMITIVE, so these
// calls go through a same-package monitor-lock wrapper (the shape of a
// multiplatform `actual` or atomicfu's JVM synchronized) to compile cleanly.
package test

fun <R> synchronized(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)

class Counter<T> {
    val count: Int = 1
    val nullableCount: Int? = null
    @JvmField
    val flag: Boolean = false
    val ratio: Double = 1.0
    val letter: Char = 'a'
    private val limit: Long = 10L
        get() = field

    /** Doc comment: attached to the declaration, not part of its type. */
    val documented: Short = 1

    fun literals() {
        <!SynchronizedOnBoxedPrimitive!>synchronized(1) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(1_000) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(1L) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(0x1FL) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(0b1L) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(1.5) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(2f) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(1e3) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(true) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized('c') { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized('\n') { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(1, { })<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(1, block = { })<!>
        <!SynchronizedOnBoxedPrimitive!>test.synchronized(1) { }<!>
    }

    fun properties() {
        <!SynchronizedOnBoxedPrimitive!>synchronized(count) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(flag) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(ratio) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(letter) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(limit) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(documented) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized<!>(
            count
        ) { }
        val local: Byte = 1
        <!SynchronizedOnBoxedPrimitive!>synchronized(local) { }<!>
        if (nullableCount != null) {
            <!SynchronizedOnBoxedPrimitive!>synchronized(nullableCount) { }<!>
        }
        run {
            <!SynchronizedOnBoxedPrimitive!>synchronized(count) { }<!>
        }
    }

    val initialized = <!SynchronizedOnBoxedPrimitive!>synchronized(count) { 2 }<!>

    companion object {
        val shared: Float = 1f

        fun fromCompanion() {
            <!SynchronizedOnBoxedPrimitive!>synchronized(shared) { }<!>
        }
    }

    fun fromOuter() {
        <!SynchronizedOnBoxedPrimitive!>synchronized(shared) { }<!>
    }
}

object Registry {
    val version: Int = 1

    fun update() {
        <!SynchronizedOnBoxedPrimitive!>synchronized(version) { }<!>
        val listener = object {
            fun onEvent() {
                <!SynchronizedOnBoxedPrimitive!>synchronized(version) { }<!>
            }
        }
        listener.onEvent()
    }
}

enum class Mode {
    A {
        override fun lock() {
            <!SynchronizedOnBoxedPrimitive!>synchronized(modeCount) { }<!>
        }
    };

    val modeCount: Int = 0

    abstract fun lock()
}

fun topLevelLiteral() {
    <!SynchronizedOnBoxedPrimitive!>synchronized(0) { }<!>
}
