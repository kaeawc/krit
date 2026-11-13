// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 11, 18, 24, 29
// Shadowing: a name reused by a lambda parameter, a nested function, a loop variable or a local function
// is a different symbol, so the outer parameter stays unused.
package test.unusedparametershadowing

fun shadowedByLambda(<!UnusedParameter!>id<!>: String) {
    listOf("x").forEach { id -> println(id) }
}

fun shadowedByNestedFunction(<!UnusedParameter!>id<!>: String) {
    fun nested(id: String) {
        println(id)
    }
    nested("local")
}

fun shadowedByForLoopVariable(<!UnusedParameter!>item<!>: String) {
    for (item in listOf("a")) {
        println(item)
    }
}

fun shadowedByLocalFunction(<!UnusedParameter!>handler<!>: () -> Unit) {
    fun handler() {}
    handler()
}

fun shadowedByCatch(<!UnusedParameter!>error<!>: String) {
    try {
        println("x")
    } catch (error: Exception) {
        println(error)
    }
}
