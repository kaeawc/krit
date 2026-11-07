package fixtures.positive.potentialbugs

import java.text.DecimalFormat
import java.text.NumberFormat

class HardcodedNumberFormat {
    fun cacheKeyDecimal(value: Double): String {
        val fmt = DecimalFormat("#,###.##")
        val cacheKey = "amount-" + fmt.format(value)
        return cacheKey
    }

    fun cacheKeyNumber(value: Double): String = "price-" + NumberFormat.getInstance().format(value)
}
