// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 12, 17, 22, 26, 32, 36, 40, 44, 51
// Positives: Currency.getInstance(Locale.getDefault()) in a class whose name contains price, money or amount (any case).
package test

import java.util.Currency
import java.util.Locale

class PriceFormatter {
    private val currency = <!LocaleDefaultForCurrency!>Currency.getInstance(Locale.getDefault())<!>

    fun symbol(): String = <!LocaleDefaultForCurrency!>Currency.getInstance(Locale.getDefault())<!>.symbol
}

class MoneyConverter {
    fun code(): String {
        return <!LocaleDefaultForCurrency!>java.util.Currency.getInstance(java.util.Locale.getDefault())<!>.currencyCode
    }
}

class TotalAmount {
    val currency: Currency = <!LocaleDefaultForCurrency!>Currency.getInstance(Locale.getDefault())<!>
}

class LowerPRICEUpper {
    fun currency() = <!LocaleDefaultForCurrency!>Currency.getInstance(Locale.getDefault())<!>
}

class PriceList {
    companion object {
        // A companion object defers to the class around it.
        val currency: Currency = <!LocaleDefaultForCurrency!>Currency.getInstance(Locale.getDefault())<!>
    }

    fun anonymous(): Any = object : Any() {
        val currency = <!LocaleDefaultForCurrency!>Currency.getInstance(Locale.getDefault())<!>
    }

    fun inLambda(block: (Currency) -> Unit) {
        block(<!LocaleDefaultForCurrency!>Currency.getInstance(Locale.getDefault())<!>)
    }

    class NestedAmount {
        val currency = <!LocaleDefaultForCurrency!>Currency.getInstance(Locale.getDefault())<!>
    }
}

class PriceCompanionOwner {
    companion object Factory {
        // A named companion defers to the class around it.
        val currency: java.util.Currency = <!LocaleDefaultForCurrency!>java.util.Currency.getInstance(java.util.Locale.getDefault())<!>
    }
}
