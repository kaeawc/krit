// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): each call below is toString() on a java.lang wrapper
// constructor, so the message is true of the code and FIR reports it. Go (like
// detekt) needs the wrapper's simple name as the callee, so it misses each one.
package test

import java.lang.Integer as JInt

// An import alias of the wrapper.
fun importAlias(x: Int): String = <!UnnecessaryTemporaryInstantiation!>JInt(x)<!>.toString()

// A typealias of the wrapper.
typealias Boxed = java.lang.Integer

fun typeAlias(x: Int): String = <!UnnecessaryTemporaryInstantiation!>Boxed(x)<!>.toString()

// A qualified constructor.
fun qualified(x: Int): String = <!UnnecessaryTemporaryInstantiation!>java.lang.Integer(x)<!>.toString()
