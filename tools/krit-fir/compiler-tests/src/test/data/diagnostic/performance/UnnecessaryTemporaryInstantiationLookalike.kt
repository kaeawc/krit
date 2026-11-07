// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
// Divergence (precision): the same-package class Integer shadows
// java.lang.Integer, so its constructor creates no wrapper and the message is
// false for this code. Go matches the constructor name alone and reports it.
package test

class Integer(val value: Int)

fun samePackageShadow(x: Int): String = Integer(x).toString()
