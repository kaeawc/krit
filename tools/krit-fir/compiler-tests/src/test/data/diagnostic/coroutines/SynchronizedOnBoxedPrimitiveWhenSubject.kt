// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positive (deliberate improvement over Go): a typed when-subject variable
// used as the lock. Kept in its own file because tree-sitter does not parse
// the when-subject declaration, which would hide the rest of a shared file
// from the Go rule.
package test

fun <R> synchronized(lock: Any, block: () -> R): R = kotlin.synchronized(lock, block)

class WhenSubject {
    fun f(x: Int) {
        when (val n: Int = x) {
            // Go misses this because tree-sitter has no property_declaration
            // for n; FIR is correct because the lock n is a boxed Int.
            else -> <!SynchronizedOnBoxedPrimitive!>synchronized(n) { }<!>
        }
    }
}
