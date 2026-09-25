// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (precision): the file imports java.text.SimpleDateFormat, but each
// call below resolves to something else named SimpleDateFormat: a local class,
// a nested class, a local function, a member function, a lambda-typed
// parameter or local, and extension or member functions called with or
// without an explicit receiver. Go reports every call spelled
// SimpleDateFormat with fewer than two arguments; none of these constructs a
// SimpleDateFormat, so FIR does not report them.
package test

import java.text.SimpleDateFormat

fun jdk(): SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>

fun localClass(): Any {
    class SimpleDateFormat(val pattern: String)
    return SimpleDateFormat("yyyy")
}

class Holder {
    class SimpleDateFormat(val pattern: String)

    fun nested(): Any = SimpleDateFormat("yyyy")
}

fun localFun(): Any {
    fun SimpleDateFormat(pattern: String): Any = pattern
    return SimpleDateFormat("yyyy")
}

class Member {
    fun SimpleDateFormat(pattern: String): Any = pattern

    fun use(): Any = SimpleDateFormat("yyyy")
}

fun invoked(SimpleDateFormat: (String) -> Any): Any = SimpleDateFormat("yyyy")

fun localLambda(): Any {
    val SimpleDateFormat = { pattern: String -> pattern }
    return SimpleDateFormat("yyyy")
}

fun String.SimpleDateFormat(): Int = length

fun extensionOnReceiver(pattern: String): Any = pattern.SimpleDateFormat()

fun implicitReceiver(x: String): Any = with(x) { SimpleDateFormat() }

object Formats {
    fun SimpleDateFormat(pattern: String): Any = pattern
}

fun memberOnReceiver(): Any = Formats.SimpleDateFormat("yyyy")
