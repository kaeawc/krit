// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25
// Constructors without a locale parameter, which Android Lint reports.
// `SimpleDateFormat(pattern, symbols)` takes the month and day names from the
// symbols but the calendar and digits from the default locale; Go reports it
// when the symbols are built in the argument. Divergence (recall): a
// superclass delegation to a constructor without a locale builds a
// SimpleDateFormat with the default locale too; Go reads only call
// expressions.
package test.locale

import java.text.DateFormatSymbols
import java.text.SimpleDateFormat
import java.util.Locale

class RootFormat(pattern: String) : <!SimpleDateFormat!>SimpleDateFormat(pattern)<!>

class DelegatingFormat : SimpleDateFormat {
    constructor(pattern: String) : <!SimpleDateFormat!>super(pattern)<!>

    constructor() : this("yyyy")
}

class Formats {
    fun withSymbols(): SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat("yyyy", DateFormatSymbols(Locale.US))<!>

    // Go does not see the type of a symbols variable.
    fun withVariable(symbols: DateFormatSymbols): SimpleDateFormat = <!SimpleDateFormat!>SimpleDateFormat("yyyy", symbols)<!>

    fun anonymousSubclass(): SimpleDateFormat = object : <!SimpleDateFormat!>SimpleDateFormat("yyyy")<!> {}

    fun subclassCall(): SimpleDateFormat = RootFormat("yyyy")
}
