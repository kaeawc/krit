package potentialbugs

import java.time.format.DateTimeFormatter
import java.util.Locale

class HardcodedDateFormat {
    val formatter: DateTimeFormatter = DateTimeFormatter.ofPattern("yyyy-MM-dd")
}
