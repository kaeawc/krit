// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 28, 31, 33, 37, 43
// Positives for SimpleDateFormat: a project class named SimpleDateFormat that
// extends a SimpleDateFormat, directly or through another class, as a
// top-level, nested, or local class. Each call builds a SimpleDateFormat and
// passes no locale, and Go reports each one by the call name, so FIR reports
// it too. A class named SimpleDateFormat that is not a SimpleDateFormat
// subtype stays unreported (SimpleDateFormatLookalike.kt,
// SimpleDateFormatShadowed.kt).
package test

import java.util.Date
import java.util.Locale

class SimpleDateFormat(pattern: String) : java.text.SimpleDateFormat(pattern)

open class RootFormat(pattern: String) : java.text.SimpleDateFormat(pattern)

class Holder {
    class SimpleDateFormat(pattern: String) : RootFormat(pattern)

    fun nested(): java.text.SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>
}

class IcuHolder {
    class SimpleDateFormat : android.icu.text.SimpleDateFormat()

    fun icu(): android.icu.text.SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat()<!>
}

fun make(): java.text.SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>

fun chained(date: Date): String = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>.format(date)

fun local(): java.text.SimpleDateFormat {
    class SimpleDateFormat(pattern: String) : java.text.SimpleDateFormat(pattern)
    return <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>
}

fun anonymousMember(): Any = object {
    inner class SimpleDateFormat(pattern: String) : java.text.SimpleDateFormat(pattern)

    val format = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>
}

// A differently named subclass is not reported, by Go or FIR (Go needs the call
// spelled SimpleDateFormat), and neither is a two-argument call.
fun root(): java.text.SimpleDateFormat = RootFormat("yyyy")

fun twoArguments(): java.text.SimpleDateFormat {
    class SimpleDateFormat(pattern: String, locale: Locale) : java.text.SimpleDateFormat(pattern, locale)
    return SimpleDateFormat("yyyy", Locale.US)
}
