// RENDER_DIAGNOSTICS_FULL_TEXT
// Where FIR resolution and the Go rule's name-based lookup disagree. FIR
// keeps every Go finding whose lock really is a boxed primitive, drops Go
// findings whose lock is not one, and adds findings Go misses only where the
// resolved lock is a primitive-typed property of the enclosing class.
package test

fun <R> synchronized(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)

class Other {
    val size: Int = 0
}

class OtherText {
    val size: String = ""
}

class WithReceiver {
    val size: Int = 1

    fun f(o: Other, t: OtherText) {
        // Go reports this because the class declares `size: Int`; the lock is
        // Other.size, also a boxed Int, so FIR keeps the finding.
        with(o) {
            <!SynchronizedOnBoxedPrimitive!>synchronized(size) { }<!>
        }
        // Go reports this because the class declares `size: Int`; FIR is
        // correct to drop it because the lock is OtherText.size, a String.
        with(t) {
            synchronized(size) { }
        }
    }
}

class SameNameOrdering {
    class Nested {
        val total: String = ""
    }

    val total: Int = 1

    fun f() {
        // Go misses this because it takes the first same-named declaration
        // (Nested.total: String); FIR is correct because the lock resolves to
        // the outer total: Int.
        <!SynchronizedOnBoxedPrimitive!>synchronized(total) { }<!>
    }
}

class Collide {
    val count = 1

    fun other() {
        val count: Long = 2L
        <!PrintlnInProduction!>println<!>(count)
    }

    fun f() {
        // Go reports this as (Long), from the typed local in other(); the
        // lock is the inferred member count, a boxed Int, so FIR keeps the
        // finding and names the lock's real type.
        <!SynchronizedOnBoxedPrimitive!>synchronized(count) { }<!>
    }
}

class ParameterShadow {
    val count: Int = 1

    // Go reports this by name; the parameter is itself a boxed Int, so FIR
    // keeps the finding.
    fun f(count: Int) {
        <!SynchronizedOnBoxedPrimitive!>synchronized(count) { }<!>
    }
}

class Locker {
    operator fun <R> invoke(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)
}

class InvokeConvention {
    val count: Int = 1

    fun f() {
        // A value named synchronized whose invoke takes an Any lock is called
        // like the function and locks on the argument, as Go assumes.
        val synchronized = Locker()
        <!SynchronizedOnBoxedPrimitive!>synchronized(1) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(count) { }<!>
    }
}

typealias Count = Int

class WrittenTypeHiddenFromGo {
    @field:JvmField
    val annotated: Int = 0
    val qualified: kotlin.Int = 1
    val aliased: Count = 1
    val lazyCount: Int by lazy { 0 }
    val computed: Int get() = 0

    fun f() {
        // Go misses these because it reads the type as the text after the
        // first `:` of the declaration: `JvmField val annotated: Int` (that
        // `:` is the annotation's use-site target), `kotlin.Int`, `Count`,
        // `Int by lazy { 0 }`, and `Int get()` are not primitive names. FIR is
        // correct because each property's written type is a primitive, so the
        // lock is a boxed primitive.
        <!SynchronizedOnBoxedPrimitive!>synchronized(annotated) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(qualified) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(aliased) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(lazyCount) { }<!>
        <!SynchronizedOnBoxedPrimitive!>synchronized(computed) { }<!>
    }
}

open class Base<T> {
    val count: Int = 1

    fun f() {
        val o = object : Base<Int>() {
            // count here is the object's inherited Base<Int>.count.
            fun g() {
                <!SynchronizedOnBoxedPrimitive!>synchronized(count) { }<!>
            }
        }
        o.g()
    }
}
