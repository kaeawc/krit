package fixtures.positive.potentialbugs

import java.text.SimpleDateFormat
import java.time.format.DateTimeFormatter

class HardcodedDateFormat {
    fun simpleDateFormat() {
        val legacy = SimpleDateFormat("yyyy-MM-dd")
        println(legacy)
    }

    fun dateTimeFormatter() {
        val modern = DateTimeFormatter.ofPattern("yyyy-MM-dd")
        println(modern)
    }
}
