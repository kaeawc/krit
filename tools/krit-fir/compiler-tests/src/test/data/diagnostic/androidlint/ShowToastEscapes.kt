// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 48, 51, 53
// A toast that escapes to code that may show it counts as shown, as in Go
// (toastResultEscapes) and Android Lint: passed as an argument (also as a
// vararg element, an implicit invoke, or a superclass constructor argument),
// returned explicitly or from an expression body or getter, or assigned to a
// member property. A local holding the toast escapes the same way through its
// reads. A lambda's last statement is only its value, so a toast left there is
// reported, as is one only compared or interpolated.
package test.showtoast.escapes

import android.content.Context
import android.widget.Toast

open class Holder(val toast: Toast)

class SuperArgument(context: Context) : Holder(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))

class Escapes(private val context: Context) {
    private var current: Toast? = null

    val getter: Toast get() = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)

    fun vararg(): List<Toast> = listOf(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))

    fun invoke(sink: (Toast) -> Unit) {
        sink.invoke(Toast.makeText(context, "Hello", Toast.LENGTH_SHORT))
    }

    fun member() {
        this.current = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
    }

    fun localPassed(sink: (Toast) -> Unit) {
        val toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
        sink(toast)
    }

    fun localReturned(): Toast {
        var toast: Toast? = null
        toast = Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
        return toast
    }

    fun labeledReturn(): Toast = run { return@run Toast.makeText(context, "Hello", Toast.LENGTH_SHORT) }

    fun lambdaValue() {
        run { <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!> }
    }

    fun compared(): Boolean = <!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!> == null

    fun interpolated(): String = "${<!ShowToast!>Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)<!>}"
}
