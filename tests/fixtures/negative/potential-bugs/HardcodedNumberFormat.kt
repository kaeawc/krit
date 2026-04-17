package fixtures.negative.potentialbugs

import java.text.DecimalFormat
import java.text.DecimalFormatSymbols
import java.text.NumberFormat
import java.util.Locale

class HardcodedNumberFormat {
    fun decimalFormat() {
        val fmt = DecimalFormat("#,###.##", DecimalFormatSymbols(Locale.ROOT))
        println(fmt)
    }

    fun numberFormat() {
        val fmt = NumberFormat.getInstance(Locale.US)
        println(fmt)
    }
}
