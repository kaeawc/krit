// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 19
// Positives for SimpleDateFormat: an import alias named SimpleDateFormat for a
// project subclass of java.text.SimpleDateFormat. The call builds a
// SimpleDateFormat and passes no locale; Go reports it by the call name, so
// FIR reports it too. The subclass called by its own name, and an alias for a
// class that is not a SimpleDateFormat, are reported by neither.
package test

import test.Formats.RootFormat as SimpleDateFormat
import test.Formats.Plain as OtherSimpleDateFormat

object Formats {
    open class RootFormat(pattern: String) : java.text.SimpleDateFormat(pattern)

    class Plain(val pattern: String)
}

fun aliased(): Formats.RootFormat = <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!>

fun direct(): Formats.RootFormat = Formats.RootFormat("yyyy")

fun plain(): Formats.Plain = OtherSimpleDateFormat("yyyy")
