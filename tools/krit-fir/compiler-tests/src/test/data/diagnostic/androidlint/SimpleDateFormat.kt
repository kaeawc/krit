// RENDER_DIAGNOSTICS_FULL_TEXT
// Positives for SimpleDateFormat: a java.text or android.icu.text
// SimpleDateFormat constructor call with fewer than two arguments formats with
// the default locale. Go reports each of these on the first line of the call.
package test

import java.text.SimpleDateFormat
import java.util.Date

class Formats {
    fun pattern(): SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat("yyyy-MM-dd")<!>

    fun noArguments(): SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat()<!>

    fun commentOnly(): SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat(/* default pattern */)<!>

    fun parameter(pattern: String): SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat(pattern)<!>

    fun qualified(): java.text.SimpleDateFormat = <!SimpleDateFormat!>java.text.SimpleDateFormat("HH:mm")<!>

    fun chained(date: Date): String = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>.format(date)

    fun applied(): SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>.apply { isLenient = false }

    fun inLambda(): SimpleDateFormat = run { <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!> }

    fun lazyFormat(): Lazy<SimpleDateFormat> = lazy { <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!> }

    fun threadLocal(): ThreadLocal<SimpleDateFormat> = ThreadLocal.withInitial { <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!> }

    fun inAnonymousObject(): Any = object {
        val format = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>
    }

    fun nested(date: Date): String {
        fun local(): SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat("MM")<!>
        return local().format(date)
    }

    // Go reports on the first line of the call expression: the package
    // qualifier of a split qualified call, and the callee of a call whose
    // arguments wrap.
    fun splitQualified(): java.text.SimpleDateFormat {
        val format = <!SimpleDateFormat!>java.text<!>
            .SimpleDateFormat("yyyy")
        return format
    }

    fun wrappedArguments(): SimpleDateFormat {
        val format =
            <!SimpleDateFormat!>SimpleDateFormat(<!>
                "yyyy-MM-dd",
            )
        return format
    }

    companion object {
        val SHARED = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>
    }
}

val topLevel = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>

// android.icu.text.SimpleDateFormat(pattern) and its no-argument constructor
// also use the default locale. Go reports them by the call name.
fun icuPattern(): android.icu.text.SimpleDateFormat = <!SimpleDateFormat!>android.icu.text.SimpleDateFormat("yyyy")<!>

fun icuNoArguments(): android.icu.text.SimpleDateFormat = <!SimpleDateFormat!>android.icu.text.SimpleDateFormat()<!>
