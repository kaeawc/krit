// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 12, 15, 17, 19, 21, 24, 26, 28
package test

fun illegalArgument(input: String) {
    if (input.isEmpty()) {
        throw <!ThrowingExceptionsWithoutMessageOrCause!>IllegalArgumentException<!>()
    }
}

fun illegalState(flag: Boolean) {
    if (flag) throw <!ThrowingExceptionsWithoutMessageOrCause!>IllegalStateException<!>()
}

fun runtime(): Nothing = throw <!ThrowingExceptionsWithoutMessageOrCause!>RuntimeException<!>()

fun general(): Nothing = throw <!ThrowingExceptionsWithoutMessageOrCause!>Exception<!>()

fun nullPointer(): Nothing = throw <!ThrowingExceptionsWithoutMessageOrCause!>NullPointerException<!>()

fun indexOutOfBounds(): Nothing = throw <!ThrowingExceptionsWithoutMessageOrCause!>IndexOutOfBoundsException<!>()

// The rule looks at every call, not only at throws.
fun notThrown(): Throwable = <!ThrowingExceptionsWithoutMessageOrCause!>RuntimeException<!>()

fun qualified(): Nothing = throw java.lang.<!ThrowingExceptionsWithoutMessageOrCause!>IllegalStateException<!>()

fun inLambda(items: List<Int>): List<Int> = items.map { if (it < 0) throw <!ThrowingExceptionsWithoutMessageOrCause!>IllegalArgumentException<!>() else it }
