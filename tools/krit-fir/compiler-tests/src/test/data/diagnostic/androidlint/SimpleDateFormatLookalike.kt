// RENDER_DIAGNOSTICS_FULL_TEXT
// Divergence (precision): a project class named SimpleDateFormat in the same
// package, with no java.text import, is not java.text.SimpleDateFormat. Go
// reports its one-argument call anyway, because it matches the call name
// alone; nothing here formats dates with the default locale, so FIR does not
// report it. The qualified JDK call still reports.
package test

class SimpleDateFormat(val pattern: String)

fun lookalike(): SimpleDateFormat = SimpleDateFormat("yyyy")

fun jdk(): java.text.SimpleDateFormat = <!SimpleDateFormat!>java.text.SimpleDateFormat("yyyy")<!>
