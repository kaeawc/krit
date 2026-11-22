// go-lines: 7, 11, 16, 17, 22
package unnecessarysafecall.basic

class SafeCallKnown

fun safeCallParameter(value: SafeCallKnown) {
    value<!UnnecessarySafeCall!>?.<!>hashCode()
}

fun safeCallDefault(value: String = "default") {
    println(value<!UnnecessarySafeCall!>?.<!>length)
}

fun safeCallLocal() {
    val name: String = "hello"
    println(name<!UnnecessarySafeCall!>?.<!>length)
    println(name /* ?.fake */ <!UnnecessarySafeCall!>?.<!>length)
}

fun safeCallMultiline(value: String) {
    println(value
        <!UnnecessarySafeCall!>?.<!>length)
}

fun safeCallNullable(value: String?) = value?.length
