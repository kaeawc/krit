// go-lines: 8, 9, 10, 16
package notnulloperator.basic

class AssertionValue

fun explicitAssertion(value: AssertionValue, name: String) {
    val local: String = "hello"
    println(local<!UnnecessaryNotNullOperator!>!!<!>)
    println(value<!UnnecessaryNotNullOperator!>!!<!>)
    println(name<!UnnecessaryNotNullOperator!>!!<!>)
}

fun multilineAssertion() {
    val text: String = "hello"
    println(text
        <!UnnecessaryNotNullOperator!>!!<!>)
}
