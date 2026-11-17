// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 19
// Positives for SimpleDateFormat: a typealias named SimpleDateFormat for a
// project subclass of java.text.SimpleDateFormat. The call builds a
// SimpleDateFormat and passes no locale; Go reports it by the call name, so
// FIR reports it too. A typealias named SimpleDateFormat for a class that is
// not a SimpleDateFormat is not reported.
package test

import java.util.Date

// The delegation passes no locale either (SimpleDateFormatLocale.kt).
open class RootFormat(pattern: String) : <!SimpleDateFormat!>java.text.SimpleDateFormat(pattern)<!>

typealias SimpleDateFormat = RootFormat

fun aliased(): RootFormat = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>

fun aliasedChained(date: Date): String = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>.format(date)

fun direct(): RootFormat = RootFormat("yyyy")
