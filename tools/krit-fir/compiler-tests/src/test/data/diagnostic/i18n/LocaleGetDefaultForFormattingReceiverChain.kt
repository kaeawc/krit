// RENDER_DIAGNOSTICS_FULL_TEXT
// Receiver chains through scope functions and extensions. Go reports every
// withLocale(Locale.getDefault()) whose receiver text starts with
// `DateTimeFormatter.ISO_`; FIR follows the chain only through calls whose
// result comes from their receiver.
package lgdffreceiverchain

import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.Locale

fun DateTimeFormatter.utc(): DateTimeFormatter = withZone(ZoneOffset.UTC)

val DateTimeFormatter.utcZone: DateTimeFormatter get() = this.withZone(ZoneOffset.UTC)

fun DateTimeFormatter.userFacing(): DateTimeFormatter = DateTimeFormatter.ofPattern("EEEE d MMMM")

// Scope functions that return their receiver: Go and FIR both report.
val also = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.also { println(it) }.withLocale(Locale.getDefault())<!>
val apply = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.apply { toString() }.withLocale(Locale.getDefault())<!>
val takeIf = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.takeIf { true }!!.withLocale(Locale.getDefault())<!>
val takeUnless = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.takeUnless { false }?.withLocale(Locale.getDefault())<!>

// let/run whose lambda returns the receiver, or a formatter derived from it:
// Go and FIR both report.
val letIt = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.let { it }.withLocale(Locale.getDefault())<!>
val letNamed = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.let { f -> f.withZone(ZoneOffset.UTC) }.withLocale(Locale.getDefault())<!>
val letStatements = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.let { println(it); it }.withLocale(Locale.getDefault())<!>
val safeLet = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT?.let { it }?.withLocale(Locale.getDefault())<!>
val runThis = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.run { this }.withLocale(Locale.getDefault())<!>
val runWithZone = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.run { withZone(ZoneOffset.UTC) }.withLocale(Locale.getDefault())<!>
val runExtension = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.run { utc() }.withLocale(Locale.getDefault())<!>

// A lambda that returns another ISO constant: still an ISO formatter.
val letOtherConstant = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.let { DateTimeFormatter.ISO_DATE }.withLocale(Locale.getDefault())<!>

// Project extensions whose body derives the formatter from the receiver:
// Go and FIR both report.
val viaExtension = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.utc().withLocale(Locale.getDefault())<!>
val viaExtensionProperty = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.utcZone.withLocale(Locale.getDefault())<!>

// An open extension may be overridden, so FIR does not read its body; it
// walks through the call to the receiver, as Go does, and both report.
open class Extensions {
    open fun DateTimeFormatter.utcish(): DateTimeFormatter = withZone(ZoneOffset.UTC)

    fun viaOpenExtension(): DateTimeFormatter =
        <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.utcish().withLocale(Locale.getDefault())<!>
}

// Go reports these because the receiver text starts with
// `DateTimeFormatter.ISO_INSTANT`; the formatter that receives withLocale is
// the user-facing ofPattern formatter the lambda or extension returns, not the
// ISO constant, so FIR does not report them.
val letPattern = DateTimeFormatter.ISO_INSTANT.let { DateTimeFormatter.ofPattern("EEEE d MMMM") }.withLocale(Locale.getDefault())
val runPattern = DateTimeFormatter.ISO_INSTANT.run { DateTimeFormatter.ofPattern("EEEE d MMMM") }.withLocale(Locale.getDefault())
val extensionPattern = DateTimeFormatter.ISO_INSTANT.userFacing().withLocale(Locale.getDefault())
