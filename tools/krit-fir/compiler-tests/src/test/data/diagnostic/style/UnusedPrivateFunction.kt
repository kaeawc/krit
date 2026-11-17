// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 6, 8, 10, 12, 14, 17, 22, 27, 32, 36, 41, 48, 53, 60, 71, 75, 82
// Positives: a private function that nothing in its file calls.
package test.unusedprivatefunction

private fun <!UnusedPrivateFunction!>topLevel<!>() {}

private fun <!UnusedPrivateFunction!>withParams<!>(a: Int, b: String): Int = a + b.length

private suspend fun <!UnusedPrivateFunction!>suspending<!>() {}

private inline fun <!UnusedPrivateFunction!>inlined<!>(block: () -> Unit) = block()

private fun String.<!UnusedPrivateFunction!>unusedExtension<!>() = this + "!"

class Members {
    private fun <!UnusedPrivateFunction!>unusedMember<!>() {}

    fun used() = 1

    companion object {
        private fun <!UnusedPrivateFunction!>unusedInCompanion<!>() {}
    }
}

object Holder {
    private fun <!UnusedPrivateFunction!>unusedInObject<!>() {}
}

class Nested {
    class Inner {
        private fun <!UnusedPrivateFunction!>unusedInNested<!>() {}
    }

    inner class Deep {
        private fun <!UnusedPrivateFunction!>unusedInInner<!>() {}
    }
}

interface Api {
    private fun <!UnusedPrivateFunction!>unusedInInterface<!>() {}
}

enum class Level {
    LOW,
    HIGH;

    private fun <!UnusedPrivateFunction!>unusedInEnum<!>() {}
}

fun local() {
    val anonymous = object {
        private fun <!UnusedPrivateFunction!>unusedInAnonymous<!>() {}
        fun go() = 1
    }
    println(anonymous.go())
}

// A function that only calls itself is unused, like Go.
private fun <!UnusedPrivateFunction!>recursive<!>(n: Int): Int = if (n == 0) 0 else recursive(n - 1)

// Two functions that only call each other are each called from outside their own text, so Go keeps both.
private fun first(n: Int): Int = if (n == 0) 0 else second(n - 1)

private fun second(n: Int): Int = if (n == 0) 0 else first(n - 1)

// A function annotated with something that is not an entry point is judged like any other.
annotation class Unrelated

@Unrelated
private fun <!UnusedPrivateFunction!>annotated<!>() {}

// A name that only shows up in a comment or a string is not a call, and Go agrees.
// uncalledInComment()
private fun <!UnusedPrivateFunction!>uncalledInComment<!>() {}

val message = "uncalledInComment"

// The finding stays on the name when annotations span several lines.
@Unrelated
@Deprecated("old")
private fun <!UnusedPrivateFunction!>multiLine<!>() {}
