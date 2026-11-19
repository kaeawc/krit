// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 19, 22
// localizedBy(Locale.getDefault()) on a machine-readable formatter. Unlike
// withLocale, localizedBy also takes the locale's numbering system and
// calendar, so on a device set to Arabic digits or a Buddhist calendar the ISO
// output changes. Go and FIR both report it.
package lgdfflocalizedby

import java.time.Instant
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.Locale

fun localized(instant: Instant): String =
    <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.localizedBy(Locale.getDefault())<!>.format(instant)

// Derived from the constant, and followed by a withLocale copy.
val derived =
    <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_OFFSET_DATE_TIME<!>.withZone(ZoneOffset.UTC).localizedBy(Locale.getDefault())

val thenWithLocale =
    <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_LOCAL_DATE<!>.localizedBy(Locale.getDefault()).withLocale(Locale.ROOT)

// Negatives: a fixed locale, a pattern formatter, the Category overload.
val root = DateTimeFormatter.ISO_LOCAL_DATE.localizedBy(Locale.ROOT)
val pattern = DateTimeFormatter.ofPattern("yyyy-MM-dd").localizedBy(Locale.getDefault())
val category = DateTimeFormatter.ISO_LOCAL_DATE.localizedBy(Locale.getDefault(Locale.Category.FORMAT))
