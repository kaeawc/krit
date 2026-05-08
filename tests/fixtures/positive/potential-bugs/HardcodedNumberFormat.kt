package fixtures.positive.potentialbugs

import java.text.DecimalFormat
import java.text.NumberFormat

class HardcodedNumberFormat {
    fun decimalFormat() {
        val fmt = DecimalFormat("#,###.##")
        println(fmt)
    }

    fun numberFormat() {
        val fmt = NumberFormat.getInstance()
        println(fmt)
    }
}
