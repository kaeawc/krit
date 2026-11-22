// go-lines: none
package notnulloperator.conservative

fun smartCastAssertion(value: String?) {
    // Go with compiler diagnostics reports; FIR deliberately ignores smart-cast-only non-nullness.
    if (value != null) println(value!!)
}

fun repeatedAssertion(value: String?) {
    // Deliberate conservative miss: unwrapping nested assertions keeps the original nullable type.
    println(value!!!!)
    // Go with compiler diagnostics reports the flow-proven second assertion; FIR keeps the nullable type.
    println(value!!)
}

fun mutableAssertion() {
    // Both fallbacks skip mutable names; FIR also skips the compiler-proven non-null mutable local.
    var text: String = "hello"
    println(text!!)
    text = "world"
    println(text)
}

fun capturedAssertion() {
    var value: String? = "hello"
    val clear = { value = null }
    println(value!!)
    clear()
}

class DottedMemberVarHolder {
    var memberText: String = "member"
}

fun dottedMemberVarAssertion(holder: DottedMemberVarHolder) {
    // Deliberate FIR miss: isVar exempts dotted member vars.
    println(holder.memberText!!)
}

class LateinitMemberVarHolder {
    lateinit var memberText: String
}

fun lateinitMemberVarAssertion(holder: LateinitMemberVarHolder) {
    // Deliberate FIR miss: isVar exempts lateinit member vars.
    println(holder.memberText!!)
}

var topLevelMutableText: String = "top-level"

fun topLevelMutableVarAssertion() {
    // Deliberate FIR miss: isVar exempts top-level vars.
    println(topLevelMutableText!!)
}

class ConstructorVarHolder(var constructorText: String)

fun constructorVarPropertyAssertion(holder: ConstructorVarHolder) {
    // Deliberate FIR miss: isVar exempts constructor var properties.
    println(holder.constructorText!!)
}
