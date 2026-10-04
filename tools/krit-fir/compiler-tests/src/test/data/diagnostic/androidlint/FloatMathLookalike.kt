// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 20, 29, 35, 40, 44
// Divergence (precision): receivers spelled `FloatMath` that are not the
// deprecated android.util.FloatMath. Go matches the receiver's text, so it
// reports every navigation below; the message ("FloatMath is deprecated") is
// not true of any of them, so FIR does not report them.
package test

// A project object named FloatMath.
object FloatMath {
    const val PI = 3.14f

    fun sqrt(x: Float): Float = x
}

// Go reports: the project object's function, not the platform class.
fun project(x: Float) = FloatMath.sqrt(x)

// Go reports: a property of the project object.
fun constant() = FloatMath.PI

class Wrapper {
    // A nested object named FloatMath.
    object FloatMath {
        fun sin(x: Float) = x
    }

    // Go reports: the nested object's function.
    fun f(x: Float) = FloatMath.sin(x)
}

// Go reports: a local variable named FloatMath, a String.
fun variable(): Int {
    val FloatMath = "text"
    return FloatMath.length
}

// Go reports: a property named FloatMath, a List.
class Holder(val FloatMath: List<Int>) {
    fun size() = FloatMath.size
}

// Go reports: a lambda parameter named FloatMath.
val lengths = listOf("a").map { FloatMath -> FloatMath.length }
