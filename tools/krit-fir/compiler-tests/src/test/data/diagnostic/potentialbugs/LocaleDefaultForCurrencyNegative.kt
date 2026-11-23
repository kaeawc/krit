// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives: the currency comes from business data, the class is not money related, the Locale is not the plain
// device default, or the call is outside any class.
package test

import java.util.Currency
import java.util.Locale

class PriceFromBusinessData(private val code: String, private val locale: Locale) {
    private val fromCode = Currency.getInstance(code)
    private val fromHeldLocale = Currency.getInstance(locale)
    private val otherLocale = Currency.getInstance(Locale.US)
    private val categoryOverload = Currency.getInstance(Locale.getDefault(Locale.Category.FORMAT))
}

class UserSettings {
    val currency: Currency = Currency.getInstance(Locale.getDefault())
}

fun topLevel(): Currency = Currency.getInstance(Locale.getDefault())

class PriceHolder {
    // The nearest class is the nested one, which is not money related.
    class Settings {
        val currency: Currency = Currency.getInstance(Locale.getDefault())
    }

    fun local() {
        class Helper {
            val currency: Currency = Currency.getInstance(Locale.getDefault())
        }
        println(Helper())
    }
}

object SettingsObject {
    val currency: Currency = Currency.getInstance(Locale.getDefault())
}
