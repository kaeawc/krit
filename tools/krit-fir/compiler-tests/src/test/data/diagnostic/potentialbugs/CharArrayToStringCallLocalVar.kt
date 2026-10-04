// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 20, 29, 36, 45, 57, 69, 74, 77, 83, 87, 93, 103, 110, 118, 127, 137, 149, 158
// A local var smart-cast by an is-check. Only an assignment that can run
// between the check and the call drops the finding.
package test

// Assigned after the call, outside any loop: the value is still a CharArray
// at the call.
fun assignedAfterCall(input: Any): String {
    var value: Any = input
    var out = ""
    if (value is CharArray) { out = <!CharArrayToStringCall!>value.toString()<!> }
    value = "x"
    return out + value
}

fun assignedAfterEarlyReturn(input: Any): String {
    var v: Any = input
    if (v !is CharArray) return ""
    val s = <!CharArrayToStringCall!>v.toString()<!>
    v = "x"
    return s + v
}

// The loop holds both the check and the call, so the assignment after the
// call runs before the next check, not between them.
fun loopHoldsCheck(items: List<Any>) {
    var v: Any = ""
    for (i in items) { if (v is CharArray) println(<!CharArrayToStringCall!>v.toString()<!>); v = i }
}

// The same inside a lambda that runs per element: the lambda runs the check
// and the call, and the assignment after the call.
fun lambdaHoldsCheck(items: List<Any>) {
    var v: Any = ""
    items.forEach { if (v is CharArray) println(<!CharArrayToStringCall!>v.toString()<!>); v = it }
}

// The check and the call run in a lambda; the declaring function's own
// assignment after it cannot run between them.
fun checkInLambda(input: Any): () -> Unit {
    var v: Any = input
    val show = {
        if (v is CharArray) {
            println(<!CharArrayToStringCall!>v.toString()<!>)
        }
    }
    v = "x"
    return show
}

// Assigned before the check.
fun assignedBeforeCheck(input: Any): String {
    var value: Any = ""
    value = input
    if (value is CharArray) {
        return <!CharArrayToStringCall!>value.toString()<!>
    }
    return ""
}

// Assigned only in the exit branch of the guard, which never reaches the call.
fun assignedInExitBranch(input: Any): String {
    var v: Any = input
    if (v !is CharArray) {
        v = "x"
        return v
    }
    return <!CharArrayToStringCall!>v.toString()<!>
}

// Declared inside a lambda or a local function: its own assignments are not
// nested.
fun declaredInLambda(input: Any) = run { var value: Any = input; value = input; if (value is CharArray) <!CharArrayToStringCall!>value.toString()<!> else "" }

fun declaredInLocalFunction(input: Any) {
    fun inner() { var v: Any = input; v = input; if (v is CharArray) println(<!CharArrayToStringCall!>v.toString()<!>) }
    inner()
}

// Declared in an initializer, outside any function.
class InitHolder(input: Any) {
    init { var v2: Any = input; if (v2 is CharArray) println(<!CharArrayToStringCall!>v2.toString()<!>) }
}

class PropertyHolder(input: Any) {
    val text: String = run { var v: Any = input; v = input; if (v is CharArray) <!CharArrayToStringCall!>v.toString()<!> else "" }
}

// A lambda created after the call cannot run before it.
fun lambdaAfterCall(input: Any): () -> Unit {
    var v: Any = input
    if (v is CharArray) println(<!CharArrayToStringCall!>v.toString()<!>)
    return { v = "x" }
}

// Negatives: the var may hold another value at the call.
// Go reports these because it keys the smart cast by name; the assignment can
// run between the check and the call, so the value may not be a CharArray.
fun loopAroundCallOnly(items: List<Any>, input: Any) {
    var v: Any = input
    if (v is CharArray) {
        for (i in items) { println(v.toString()); v = i }
    }
}

fun loopAroundCallOnlyEarlyReturn(items: List<Any>, input: Any) {
    var v: Any = input
    if (v !is CharArray) return
    for (i in items) { println(v.toString()); v = i }
}

fun lambdaBeforeCall(input: Any) {
    var v: Any = input
    val reset = { v = "x" }
    if (v is CharArray) {
        reset()
        println(v.toString())
    }
}

fun localFunctionBeforeCall(input: Any) {
    var v: Any = input
    fun reset() { v = "x" }
    if (v is CharArray) {
        reset()
        println(v.toString())
    }
}

fun siblingLambdaBeforeCall(input: Any) {
    var v: Any = input
    val reset = { v = "x" }
    val show = {
        if (v is CharArray) {
            reset()
            println(v.toString())
        }
    }
    show()
}

fun lambdaInLoopAroundCall(items: List<Any>, input: Any) {
    var v: Any = input
    val resets = mutableListOf<() -> Unit>()
    for (i in items) {
        if (v is CharArray) {
            resets.forEach { it() }
            println(v.toString())
        }
        resets.add { v = i }
    }
}

fun reassignedInGuardElse(input: Any): String {
    var v: Any = input
    if (v !is CharArray) return "" else v = "x"
    return v.toString()
}
