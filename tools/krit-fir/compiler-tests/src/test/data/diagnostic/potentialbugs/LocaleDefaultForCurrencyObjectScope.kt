// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
// Objects defer to the nearest class, exactly as Go does; resolution does not prove a wider name heuristic.
package test.objectscope

import java.util.Currency
import java.util.Locale

class PriceObjectScope {
    object Settings {
        val currency = <!LocaleDefaultForCurrency!>Currency.getInstance(Locale.getDefault())<!>
    }
}

class SettingsObjectScope {
    object Money {
        val currency = Currency.getInstance(Locale.getDefault())
    }
}

object AmountStandalone {
    val currency = Currency.getInstance(Locale.getDefault())
}
