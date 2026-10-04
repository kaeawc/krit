// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 18, 23, 24, 25, 28, 32, 35, 36, 37, 41x2, 44, 47, 52, 57, 61, 65, 66, 70
// Positive: withLocale(Locale.getDefault()) on a formatter rooted at a
// DateTimeFormatter ISO_* / RFC_* / BASIC_ISO_* constant, in every container
// Go visits. Go and FIR both report each call on the line where the call's
// receiver chain starts.
package lgdffpositive

import java.time.Instant
import java.time.ZoneOffset
import java.time.format.DateTimeFormatter
import java.util.Locale

fun topLevel(instant: Instant): String =
    <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault())<!>.format(instant)

fun chained(instant: Instant): String =
    <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT<!>
        .withLocale(Locale.getDefault())
        .format(instant)

// Every family of machine-readable constants Go matches.
val rfc = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.RFC_1123_DATE_TIME.withLocale(Locale.getDefault())<!>
val basic = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.BASIC_ISO_DATE.withLocale(Locale.getDefault())<!>
val localDateTime = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_LOCAL_DATE_TIME.withLocale(Locale.getDefault())<!>

// Fully qualified receiver and argument.
val qualified = <!LocaleGetDefaultForFormatting!>java.time.format.DateTimeFormatter.ISO_DATE.withLocale(java.util.Locale.getDefault())<!>

// A formatter derived from the constant is still the ISO formatter: Go matches
// the receiver text prefix, FIR walks the receiver chain to its root.
val derived = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withZone(ZoneOffset.UTC).withLocale(Locale.getDefault())<!>

// Safe calls and !! on the platform-typed constant.
val safe = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT?.withLocale(Locale.getDefault())<!>
val safeDerived = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT?.withZone(ZoneOffset.UTC)?.withLocale(Locale.getDefault())<!>
val notNull = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT!!.withLocale(Locale.getDefault())<!>

// Two withLocale calls in one chain: the receiver of each starts with the
// constant, so Go and FIR report both (the golden compares lines).
val twice = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault()).withLocale(Locale.getDefault())<!>

class Api {
    private val formatter = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_OFFSET_DATE_TIME.withLocale(Locale.getDefault())<!>

    fun member(instant: Instant): String {
        val f = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault())<!>
        return f.format(instant) + formatter.hashCode()
    }

    companion object {
        val SHARED = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_DATE_TIME.withLocale(Locale.getDefault())<!>
    }
}

object Holder {
    val formatter = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_LOCAL_DATE.withLocale(Locale.getDefault())<!>
}

interface Formats {
    fun iso(): DateTimeFormatter = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_WEEK_DATE.withLocale(Locale.getDefault())<!>
}

fun nested(instants: List<Instant>): List<String> = instants.map { instant ->
    fun local(): DateTimeFormatter = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_TIME.withLocale(Locale.getDefault())<!>
    local().format(instant) + <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_INSTANT.withLocale(Locale.getDefault())<!>.format(instant)
}

val anonymous = object {
    val formatter = <!LocaleGetDefaultForFormatting!>DateTimeFormatter.ISO_ORDINAL_DATE.withLocale(Locale.getDefault())<!>
}
