// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 13, 14
// Positives: toString() called directly on a java.lang.Integer constructor
// call, as in detekt 1.23.8. Go reports each of these too, on the constructor
// call.
package test

fun convert(x: Int): String {
    return <!UnnecessaryTemporaryInstantiation!>Integer(x)<!>.toString()
}

fun contexts(x: Int): String {
    val t = "v=${<!UnnecessaryTemporaryInstantiation!>Integer(x)<!>.toString()}"
    println(<!UnnecessaryTemporaryInstantiation!>Integer(x)<!>.toString())
    return t
}
