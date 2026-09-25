// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: shapes neither Go nor FIR reports.
package lgdffnegative

import java.time.Instant
import java.time.format.DateTimeFormatter
import java.time.format.FormatStyle
import java.util.Locale

// A locale-independent locale.
val root = DateTimeFormatter.ISO_INSTANT.withLocale(Locale.ROOT)
val us = DateTimeFormatter.ISO_INSTANT.withLocale(Locale.US)

// The Locale.Category overload.
val category = DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault(Locale.Category.FORMAT))

// A user-facing formatter may use the device locale.
val pattern = DateTimeFormatter.ofPattern("EEEE d MMMM").withLocale(Locale.getDefault())
val localized = DateTimeFormatter.ofLocalizedDate(FormatStyle.LONG).withLocale(Locale.getDefault())

// The default locale held in a variable: Go only matches the call spelled in
// the argument, and FIR does the same.
fun viaVariable(instant: Instant): String {
    val locale = Locale.getDefault()
    return DateTimeFormatter.ISO_INSTANT.withLocale(locale).format(instant)
}

// The formatter held in a variable: Go only matches the constant spelled at
// the root of the receiver, and FIR does the same.
fun viaFormatter(instant: Instant): String {
    val formatter = DateTimeFormatter.ISO_INSTANT
    return formatter.withLocale(Locale.getDefault()).format(instant)
}

// An implicit receiver: Go needs a navigation expression, and FIR needs an
// explicit receiver.
fun implicit(): DateTimeFormatter = with(DateTimeFormatter.ISO_INSTANT) { withLocale(Locale.getDefault()) }

// The ISO constant is not the root of the receiver chain.
fun notRoot(formats: List<DateTimeFormatter>): DateTimeFormatter =
    listOf(DateTimeFormatter.ISO_INSTANT).plus(formats).first().withLocale(Locale.getDefault())

// Other calls next to the default locale.
fun otherCalls(instant: Instant): String =
    DateTimeFormatter.ISO_INSTANT.format(instant) + Locale.getDefault().toString()

// A project withLocale on another type.
class Box {
    fun withLocale(locale: Locale): Box = this
}

val box = Box().withLocale(Locale.getDefault())
