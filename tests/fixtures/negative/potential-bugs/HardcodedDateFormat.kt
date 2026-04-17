package fixtures.negative.potentialbugs

import java.text.SimpleDateFormat
import java.time.format.DateTimeFormatter
import java.util.Locale

class HardcodedDateFormat {
    fun simpleDateFormat() {
        val legacy = SimpleDateFormat("yyyy-MM-dd", Locale.ROOT)
        println(legacy)
    }

    fun dateTimeFormatter() {
        val modern = DateTimeFormatter.ofPattern("yyyy-MM-dd", Locale.US)
        println(modern)
    }
}
