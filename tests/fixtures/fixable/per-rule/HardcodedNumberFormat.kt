package potentialbugs

import java.text.NumberFormat
import java.util.Locale

class HardcodedNumberFormat {
    fun cacheKey(value: Double): String = "price-" + NumberFormat.getInstance().format(value)
}
