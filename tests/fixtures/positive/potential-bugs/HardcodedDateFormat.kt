package fixtures.positive.potentialbugs

import java.time.format.DateTimeFormatter

class HardcodedDateFormat {
    fun dateTimeFormatter() {
        val modern = DateTimeFormatter.ofPattern("yyyy-MM-dd")
        println(modern)
    }
}
