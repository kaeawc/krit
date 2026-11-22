// go-lines: 19
package notnulloperator.resolution

typealias AssertionText = String
class AssertionHolder(val text: String)
fun assertionText(): String = "hello"

fun resolvedAssertions(holder: AssertionHolder, values: List<String>) {
    // Go skips dotted operands; the resolved property signature proves non-nullness.
    println(holder.text<!UnnecessaryNotNullOperator!>!!<!>)
    // Go skips indexed operands; List<String>.get returns a definitely non-null String.
    println(values[0]<!UnnecessaryNotNullOperator!>!!<!>)
    // Go cannot resolve a call as a bare name; its return signature proves non-nullness.
    println(assertionText()<!UnnecessaryNotNullOperator!>!!<!>)
    // Go skips member-access initializers; FIR has a non-null inferred local type.
    val inferred = holder.text
    println(inferred<!UnnecessaryNotNullOperator!>!!<!>)
    val alias: AssertionText = "hello"
    println(alias<!UnnecessaryNotNullOperator!>!!<!>)
}

fun <Value : Any> boundedAssertion(value: Value) {
    // Go skips generic types; the non-null Any bound proves this assertion redundant.
    println(value<!UnnecessaryNotNullOperator!>!!<!>)
}
