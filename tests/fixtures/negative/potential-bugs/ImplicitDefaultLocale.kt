package fixtures.negative.potentialbugs

import java.util.Locale

class ImplicitDefaultLocale {
    fun convert(str: String): String {
        return str.toLowerCase(Locale.US)
    }

    fun upper(str: String): String {
        return str.toUpperCase(Locale.ROOT)
    }

    // Kotlin 1.5+ `lowercase()` / `uppercase()` (no args) are locale-invariant
    // (they delegate to Locale.ROOT) and must not be flagged.
    fun invariantLower(str: String): String = str.lowercase()
    fun invariantUpper(str: String): String = str.uppercase()

    fun formatStringWithLocale() {
        String.format(Locale.US, "%d", 1)
        String.format(Locale.US, "%,d", 1000)
        String.format(Locale.US, "%.2f", 1.0)
        String.format("%s", "name")
        String.format("progress %% done")
    }

    fun formatExtensionWithLocale() {
        "%d".format(Locale.US, 1)
        "%.2f".format(Locale.US, 1.0)
        "%s".format("name")
        "%%d".format()
    }
}
