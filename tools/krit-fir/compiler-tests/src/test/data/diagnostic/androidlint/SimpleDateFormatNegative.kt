// RENDER_DIAGNOSTICS_FULL_TEXT
// Negatives for SimpleDateFormat: every call with two or more arguments, a
// subclass constructor, superclass delegation, and a constructor reference.
// Go reports none of these either: it accepts any second argument, and
// superclass delegation and `::SimpleDateFormat` are not call expressions of
// SimpleDateFormat.
package test

import java.text.DateFormatSymbols
import java.text.SimpleDateFormat
import java.util.Locale

class RootFormat(pattern: String) : SimpleDateFormat(pattern)

class DelegatingFormat : SimpleDateFormat {
    constructor(pattern: String) : super(pattern)
}

class Formats {
    fun withLocale(): SimpleDateFormat = SimpleDateFormat("yyyy-MM-dd", Locale.US)

    fun withRoot(): SimpleDateFormat = SimpleDateFormat("yyyy", Locale.ROOT)

    fun withDefault(): SimpleDateFormat = SimpleDateFormat("yyyy", Locale.getDefault())

    fun withSymbols(): SimpleDateFormat = SimpleDateFormat("yyyy", DateFormatSymbols(Locale.US))

    fun qualifiedWithLocale(): java.text.SimpleDateFormat = java.text.SimpleDateFormat("yyyy", Locale.US)

    fun wrappedWithLocale(): SimpleDateFormat =
        SimpleDateFormat(
            "yyyy",
            Locale.US,
        )

    fun icuWithLocale(): android.icu.text.SimpleDateFormat = android.icu.text.SimpleDateFormat("yyyy", Locale.US)

    fun subclass(): SimpleDateFormat = RootFormat("yyyy")

    fun delegating(): SimpleDateFormat = DelegatingFormat("yyyy")

    fun anonymousSubclass(): SimpleDateFormat = object : SimpleDateFormat("yyyy") {}

    fun reference(patterns: List<String>): List<SimpleDateFormat> = patterns.map(::SimpleDateFormat)

    fun factories(): Any = listOf(java.text.DateFormat.getDateInstance(), java.text.DateFormat.getTimeInstance())

    fun reconfigured(format: SimpleDateFormat) {
        format.applyPattern("yyyy")
    }
}
