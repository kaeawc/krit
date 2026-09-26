// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19, 32, 42
// Divergence (precision): these `Toast` classes are project and local
// declarations, not android.widget.Toast, so FIR does not report them. Go
// matches the receiver spelled `Toast` and drops these calls only when the
// Kotlin oracle resolves them elsewhere; without it (as here) it reports each
// unshown call.
package test.showtoast.lookalike

class Toast {
    fun show() {}

    companion object {
        fun makeText(message: String, duration: Int): Toast = Toast()
    }
}

fun projectToast() {
    Toast.makeText("Hello", 0)
}

fun projectToastShown() {
    Toast.makeText("Hello", 0).show()
}

class Screen {
    object Toast {
        fun makeText(message: String): String = message
    }

    fun nested() {
        Toast.makeText("Hello")
    }
}

class Formatter {
    fun makeText(message: String): String = message
}

fun localValue() {
    val Toast = Formatter()
    Toast.makeText("Hello")
}
