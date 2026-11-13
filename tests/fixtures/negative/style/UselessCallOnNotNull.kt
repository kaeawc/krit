// fir-parity: skip the fixture references an undeclared ExternalThing so the Go resolver cannot prove its argument non-null, which never compiles cleanly; the FIR negatives are covered by compiler-tests data
package fixtures.negative.style

fun safe(nullableStr: String?): String {
    return nullableStr?.orEmpty() ?: ""
}

// Navigation with nullable chain — safe call means receiver might be null
fun safeNavigation(obj: String?): String {
    return obj?.orEmpty() ?: ""
}

fun unresolvedArgument(value: ExternalThing) {
    println(listOfNotNull(value))
}
