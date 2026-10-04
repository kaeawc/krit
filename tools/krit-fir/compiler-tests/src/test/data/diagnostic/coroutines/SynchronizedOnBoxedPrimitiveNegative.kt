// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 48, 49, 54, 55, 82, 83
// Negative: locks that are not a bare boxed-primitive literal or a
// primitive-typed property declared in the enclosing class/object must not
// trigger SynchronizedOnBoxedPrimitive. Primitive locks go through a
// same-package monitor-lock wrapper, because K2 rejects kotlin.synchronized
// on a primitive (SYNCHRONIZED_BLOCK_ON_VALUE_CLASS_OR_PRIMITIVE).
package test

fun <R> synchronized(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)

val topLevelCount: Int = 1

class Service(val ctorCount: Int) {
    private val lock = Any()
    private val name: String = "service"
    val inferred = 1
    val counts: List<Int> = listOf(1)
    val shadowed: Int = 1

    fun locks(param: Int) {
        synchronized(lock) { }
        synchronized(name) { }
        synchronized(this) { }
        synchronized(0x1F) { }
        synchronized(0b101) { }
        synchronized(1u) { }
        synchronized(1uL) { }
        synchronized(-1) { }
        synchronized((1)) { }
        synchronized(1 + 1) { }
        synchronized(lock = 1) { }
        synchronized(block = { }, lock = 1)
        synchronized(this.shadowed) { }
        synchronized(param) { }
        synchronized(ctorCount) { }
        synchronized(topLevelCount) { }
        synchronized(inferred) { }
        synchronized(counts) { }
        kotlin.synchronized(lock) { }
    }

    // A parameter or a local that shadows a boxed-primitive property is not
    // that property. Go reports all four calls below because it looks the lock
    // up by name among the class's properties; FIR is correct to drop them
    // because the lock is an Any.
    fun shadowedByParameter(shadowed: Any) {
        kotlin.synchronized(shadowed) { }
        synchronized(shadowed) { }
    }

    fun shadowedByLocal() {
        val shadowed = Any()
        kotlin.synchronized(shadowed) { }
        synchronized(shadowed) { }
    }

    class Nested {
        // The property lives in the outer class, not the nearest one.
        fun fromNested(service: Service) {
            with(service) {
                synchronized(shadowed) { }
            }
        }
    }
}

fun topLevel() {
    val count: Int = 1
    synchronized(count) { }
}

// Local lookalike: its first parameter is not a monitor object. Go reports
// both calls because it matches the call by the name synchronized; FIR is
// correct to drop them because this synchronized takes a count, not a lock.
object Lookalike {
    val count: Int = 1

    fun <R> synchronized(times: Int, block: () -> R): R = block()

    fun use() {
        synchronized(count) { }
        synchronized(1) { }
    }
}
