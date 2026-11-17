// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: a local class and a nested class named like a wrapper shadow
// java.lang, so their constructors create no wrapper and the message would be
// false of this code. detekt 1.23.8 matches the name alone and reports both;
// Go skips a wrapper name the file declares.
package test

fun local(x: Int): String {
    class Integer(val v: Int)
    return Integer(x).toString()
}

object Holder {
    class Double(val d: kotlin.Double)

    fun nested(d: kotlin.Double): String = Double(d).toString()
}
