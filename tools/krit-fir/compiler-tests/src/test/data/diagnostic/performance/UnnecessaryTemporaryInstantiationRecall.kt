// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergence (recall): each call below is toString() on a java.lang wrapper
// conversion, so the message is true of the code and FIR reports it. Go needs
// the wrapper's literal name as the last qualifier segment of the conversion
// call, and that call as the direct receiver of toString, so it misses each
// one.
package test

import java.lang.Integer as JInt
import java.lang.Integer.parseInt
import java.lang.Integer.valueOf

// An import alias of the wrapper.
fun importAlias(x: Int): String = <!UnnecessaryTemporaryInstantiation!>JInt.valueOf(x).toString()<!>

// A statically imported conversion.
fun staticImport(x: Int, s: String): String =
    <!UnnecessaryTemporaryInstantiation!>valueOf(x).toString()<!> + <!UnnecessaryTemporaryInstantiation!>parseInt(s).toString()<!>

// A typealias of the wrapper.
typealias Boxed = java.lang.Integer

fun typeAlias(x: Int): String = <!UnnecessaryTemporaryInstantiation!>Boxed.valueOf(x).toString()<!>

// A parenthesized conversion.
fun parenthesized(x: Int): String = <!UnnecessaryTemporaryInstantiation!>(java.lang.Integer.valueOf(x)).toString()<!>
