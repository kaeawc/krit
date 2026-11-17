// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13
package test

fun doWork() {}

typealias Parsing = NumberFormatException

// Go matches the written type text only, so it reports this catch although the alias is an ignored type.
fun aliasOfIgnoredType() {
    try {
        doWork()
    } catch (e: Parsing) {
        println("parse")
    }
}

// Go counts the lambda parameter `e` as a use of the caught variable, so it misses this swallow.
fun shadowedByLambdaParameter() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: Exception) {
        listOf(1).forEach { e -> println(e) }
    }
}

// Go counts the local `e` as a use of the caught variable, so it misses this swallow.
fun shadowedByLocal() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: Exception) {
        val e = 1
        println(e)
    }
}

class Holder(val e: String)

// Go counts the member name `e` in `h.e` as a use of the caught variable, so it misses this swallow.
fun sameNamedMember(h: Holder) {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: Exception) {
        println(h.e)
    }
}
