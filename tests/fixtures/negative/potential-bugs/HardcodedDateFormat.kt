package fixtures.negative.potentialbugs

import java.time.format.DateTimeFormatter
import java.util.Locale

class HardcodedDateFormat {
    fun dateTimeFormatter() {
        val modern = DateTimeFormatter.ofPattern("yyyy-MM-dd", Locale.US)
        println(modern)
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
