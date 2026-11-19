// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): toString() on each java.lang wrapper constructor,
// qualified because the simple names other than Integer mean the kotlin
// types, and on a parenthesized constructor call. The message is true of each
// one. Go (like detekt) needs the wrapper's simple name as the receiver of
// `.toString()`, so it misses all of them.
package test

fun boolean(x: Boolean): String = <!UnnecessaryTemporaryInstantiation!>java.lang.Boolean(x)<!>.toString()

fun byte(x: Byte): String = <!UnnecessaryTemporaryInstantiation!>java.lang.Byte(x)<!>.toString()

fun short(x: Short): String = <!UnnecessaryTemporaryInstantiation!>java.lang.Short(x)<!>.toString()

fun long(x: Long): String = <!UnnecessaryTemporaryInstantiation!>java.lang.Long(x)<!>.toString()

fun float(x: Float): String = <!UnnecessaryTemporaryInstantiation!>java.lang.Float(x)<!>.toString()

fun double(x: Double): String = <!UnnecessaryTemporaryInstantiation!>java.lang.Double(x)<!>.toString()

fun parenthesized(x: Int): String = (<!UnnecessaryTemporaryInstantiation!>Integer(x)<!>).toString()

// Not a wrapper of a primitive.
fun character(c: Char): String = java.lang.Character(c).toString()
