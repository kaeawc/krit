// RENDER_DIAGNOSTICS_FULL_TEXT
// Lookalikes: a project `Locale.getDefault()` is not the device default
// locale, and a project `DateTimeFormatter.ISO_*` holder is not a java.time
// ISO constant, so FIR does not report them.
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
