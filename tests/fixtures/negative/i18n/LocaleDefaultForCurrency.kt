package test

import java.util.Currency

class PriceFormatter(private val currencyCode: String) {
    private val currency = Currency.getInstance(currencyCode)
}
