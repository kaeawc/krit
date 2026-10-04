// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
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

// A formatter from a function parameter, a var, or a function call: FIR
// cannot trace them to a constant, and Go only matches the constant spelled at
// the root of the receiver.
fun viaParameter(formatter: DateTimeFormatter): DateTimeFormatter = formatter.withLocale(Locale.getDefault())

fun viaVar(instant: Instant): String {
    var formatter = DateTimeFormatter.ISO_INSTANT
    formatter = DateTimeFormatter.ofPattern("EEEE d MMMM")
    return formatter.withLocale(Locale.getDefault()).format(instant)
}

fun isoFormatter(): DateTimeFormatter = DateTimeFormatter.ISO_INSTANT
val viaFunction = isoFormatter().withLocale(Locale.getDefault())

// An open property may be overridden with a user-facing formatter, so FIR
// does not trace its initializer; Go does not match the receiver text either.
open class Formats {
    open val iso: DateTimeFormatter = DateTimeFormatter.ISO_INSTANT
}

class UserFormats : Formats() {
    override val iso: DateTimeFormatter = DateTimeFormatter.ofPattern("EEEE d MMMM")
}

fun viaOpenProperty(formats: Formats): DateTimeFormatter = formats.iso.withLocale(Locale.getDefault())

// A static factory is a new formatter, not the receiver chain's constant.
val factory = DateTimeFormatter.ofPattern("yyyy").withLocale(Locale.getDefault())

// A formatter built inside `with`: Go needs the constant at the start of the
// receiver text, and FIR does not trace the implicit receiver of `with`.
val inWith = with(DateTimeFormatter.ISO_INSTANT) { withZone(java.time.ZoneOffset.UTC).withLocale(Locale.getDefault()) }

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
