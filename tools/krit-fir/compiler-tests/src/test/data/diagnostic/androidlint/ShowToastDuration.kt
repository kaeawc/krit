// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 16, 17
// A literal duration is reported on the argument, shown or not, as Android
// Lint's ToastDetector reports it ("a custom duration value is not
// supported"): Toast reads only LENGTH_SHORT or LENGTH_LONG. Parentheses are
// skipped. A named constant, a variable, and a negative number (a prefix
// expression, not a literal) are not literals. A bare toast with a literal
// duration gets both findings (NarrowedRulesFileLocalTest.showToast).
package test.showtoast.duration

import android.content.Context
import android.widget.Toast

fun durations(context: Context, duration: Int) {
    Toast.makeText(context, "Hello", <!ShowToast!>2000<!>).show()
    Toast.makeText(context, 1, <!ShowToast!>5<!>).show()
    Toast.makeText(context, "Hello", (<!ShowToast!>0<!>)).show()
    Toast.makeText(context, "Hello", -1).show()
    Toast.makeText(context, "Hello", Toast.LENGTH_LONG).show()
    Toast.makeText(context, "Hello", duration).show()
}
