package fixtures.positive.potentialbugs

import java.time.format.DateTimeFormatter

class HardcodedDateFormat {
    fun dateTimeFormatter() {
        // Month and day names in a stored file name depend on the device locale.
        val backupFileFormatter = DateTimeFormatter.ofPattern("EEE-MMM-d-yyyy")
        println(backupFileFormatter)
    }
}
