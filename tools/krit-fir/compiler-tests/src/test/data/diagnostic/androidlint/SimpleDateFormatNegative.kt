// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives for SimpleDateFormat: every constructor with a Locale parameter, a
// subclass constructor named otherwise, a superclass delegation with a
// locale, and a constructor reference. Go reports none of these either.
package test

import java.text.SimpleDateFormat
import java.util.Locale

class RootFormat(pattern: String) : SimpleDateFormat(pattern, Locale.ROOT)

class DelegatingFormat : SimpleDateFormat {
    constructor(pattern: String) : super(pattern, Locale.US)
}

class Formats {
    fun withLocale(): SimpleDateFormat = SimpleDateFormat("yyyy-MM-dd", Locale.US)

    fun withRoot(): SimpleDateFormat = SimpleDateFormat("yyyy", Locale.ROOT)

    fun withDefault(): SimpleDateFormat = SimpleDateFormat("yyyy", Locale.getDefault())

    fun qualifiedWithLocale(): java.text.SimpleDateFormat = java.text.SimpleDateFormat("yyyy", Locale.US)

    fun wrappedWithLocale(): SimpleDateFormat =
        SimpleDateFormat(
            "yyyy",
            Locale.US,
        )

    fun icuWithLocale(): android.icu.text.SimpleDateFormat = android.icu.text.SimpleDateFormat("yyyy", Locale.US)

    fun subclass(): SimpleDateFormat = RootFormat("yyyy")

    fun delegating(): SimpleDateFormat = DelegatingFormat("yyyy")

    fun anonymousSubclass(): SimpleDateFormat = object : SimpleDateFormat("yyyy", Locale.US) {}

    fun reference(patterns: List<String>): List<SimpleDateFormat> = patterns.map(::SimpleDateFormat)

    fun factories(): Any = listOf(java.text.DateFormat.getDateInstance(), java.text.DateFormat.getTimeInstance())

    fun reconfigured(format: SimpleDateFormat) {
        format.applyPattern("yyyy")
    }
}
