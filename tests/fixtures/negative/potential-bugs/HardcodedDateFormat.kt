package fixtures.negative.potentialbugs

import java.time.format.DateTimeFormatter
import java.util.Locale

class HardcodedDateFormat {
    fun dateTimeFormatter() {
        val modern = DateTimeFormatter.ofPattern("yyyy-MM-dd", Locale.US)
        println(modern)
    }

    fun numericPattern() {
        // Numeric-only patterns do not vary by locale.
        val iso = DateTimeFormatter.ofPattern("yyyy-MM-dd")
        println(iso)
    }

    fun displayNames() {
        // Month and day names shown to the user follow the device locale on purpose.
        val label = DateTimeFormatter.ofPattern("EEE, MMM d, yyyy")
        println(label)
    }

    fun localeChained() {
        val chained = DateTimeFormatter.ofPattern("EEE, MMM d").withLocale(Locale.US)
        println(chained)
    }

    fun localLookalike() {
        // Local symbol named DateTimeFormatter.ofPattern must not trigger
        // because java.time.format.DateTimeFormatter is not imported.
        val v = LocalDateTimeFormatter.ofPattern("yyyy-MM-dd")
        println(v)
    }
}

object LocalDateTimeFormatter {
    fun ofPattern(p: String): String = p
}
