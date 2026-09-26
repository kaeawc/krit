// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 23, 25, 28
// Divergence (precision): each receiver below is only named like a JDK
// wrapper. The same-package object Integer shadows java.lang.Integer, and its
// valueOf returns the Int it is given, so no wrapper is instantiated and the
// message is false for this code. Go matches the last qualifier segment of the
// call (`Integer`, `Long`) and the method name alone, so it reports each one.
package test

object Integer {
    fun valueOf(x: Int): Int = x
    fun parseInt(s: String): Int = s.length
}

object Holder {
    object Long {
        fun valueOf(x: kotlin.Long): kotlin.Long = x
    }
}

fun samePackageShadow(x: Int): String = Integer.valueOf(x).toString()

fun samePackageParse(s: String): String = Integer.parseInt(s).toString()

fun nestedObject(x: kotlin.Long): String = Holder.Long.valueOf(x).toString()

// A real java.lang.Integer conversion still reports next to the lookalikes.
fun real(x: Int): String = <!UnnecessaryTemporaryInstantiation!>java.lang.Integer.valueOf(x).toString()<!>
