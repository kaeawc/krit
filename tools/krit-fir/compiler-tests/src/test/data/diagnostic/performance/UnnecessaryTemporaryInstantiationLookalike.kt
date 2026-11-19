// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: the same-package class Integer shadows java.lang.Integer, so its
// constructor creates no wrapper and the message would be false for this
// code. Go skips a wrapper name the file declares.
package test

class Integer(val value: Int)

fun samePackageShadow(x: Int): String = Integer(x).toString()
