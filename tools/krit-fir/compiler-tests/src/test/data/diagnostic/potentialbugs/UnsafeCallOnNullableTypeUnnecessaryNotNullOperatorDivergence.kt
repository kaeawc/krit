// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 14
package test

class KnownThing

// UnnecessaryNotNullOperator.kt line 6: x has declared non-null type String.
fun literalLocal(x: String = "hello") {
    val y = x!!
}

// UnnecessaryNotNullOperator.kt line 10: value has declared non-null type KnownThing.
fun resolvedType(value: KnownThing) {
    println(value!!)
}
