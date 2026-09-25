// RENDER_DIAGNOSTICS_FULL_TEXT
// Lookalikes: a project `Locale.getDefault()` is not the device default
// locale, and a project `DateTimeFormatter.ISO_*` holder is a java.time ISO
// formatter only when it holds the java.time constant.
package lgdfflookalike

import java.time.Instant

object Locale {
    fun getDefault(): java.util.Locale = java.util.Locale.ROOT
}

object DateTimeFormatter {
    val ISO_INSTANT: java.time.format.DateTimeFormatter = java.time.format.DateTimeFormatter.ofPattern("EEEE d MMMM")
}

// Go reports this because the argument is spelled `Locale.getDefault()`; it is
// the project Locale.getDefault(), which returns Locale.ROOT.
fun projectLocale(instant: Instant): String =
    java.time.format.DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault()).format(instant)

// Go reports this because the receiver is spelled `DateTimeFormatter.ISO_`
// and the file mentions java.time.format.DateTimeFormatter; the receiver is
// the project holder's user-facing pattern formatter, not the ISO constant.
fun projectFormatter(instant: Instant): String =
    DateTimeFormatter.ISO_INSTANT.withLocale(java.util.Locale.getDefault()).format(instant)

// A project holder that forwards the java.time constants, directly or through
// a getter: the receiver is the ISO formatter, so Go and FIR both report.
object Forwarding {
    object DateTimeFormatter {
        val ISO_INSTANT: java.time.format.DateTimeFormatter = java.time.format.DateTimeFormatter.ISO_INSTANT
        val ISO_DATE: java.time.format.DateTimeFormatter get() = java.time.format.DateTimeFormatter.ISO_DATE
    }

    fun forwardingHolder(instant: Instant): String =
        <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(java.util.Locale.getDefault())<!>.format(instant)

    fun forwardingGetter(instant: Instant): String =
        <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_DATE.withLocale(java.util.Locale.getDefault())<!>.format(instant)
}
