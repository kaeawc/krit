// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 24
// Negatives Go reports: the project's own Currency and Locale are not the JDK classes. Go matches the written texts
// `Currency.getInstance(Locale.getDefault())` and reports both.
package test

class Currency private constructor() {
    companion object {
        fun getInstance(locale: Locale): Currency = Currency()
    }
}

class Locale {
    companion object {
        fun getDefault(): Locale = Locale()
    }
}

class PriceLookalike {
    val own = Currency.getInstance(Locale.getDefault())
}

class MoneyMixed {
    val jdk = <!LocaleDefaultForCurrency!>java.util.Currency.getInstance(java.util.Locale.getDefault())<!>
}
