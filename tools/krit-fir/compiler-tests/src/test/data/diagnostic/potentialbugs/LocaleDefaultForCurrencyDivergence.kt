// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Divergences from Go, which matches the written texts `Currency.getInstance` and `Locale.getDefault()` and takes only
// `class_declaration` ancestors as the enclosing class.
package test

import java.util.Currency as Money
import java.util.Locale as Region

typealias Cur = java.util.Currency

class PriceAliases {
    // Positives Go misses: an import alias and a type alias of the JDK classes.
    val aliased: Money = <!LocaleDefaultForCurrency!>Money.getInstance(Region.getDefault())<!>
    val typeAliased: Cur = <!LocaleDefaultForCurrency!>Cur.getInstance(java.util.Locale.getDefault())<!>
    // Go misses parentheses around the argument; resolution still proves the zero-argument JDK default locale.
    val parenthesized: Cur = <!LocaleDefaultForCurrency!>java.util.Currency.getInstance((java.util.Locale.getDefault()))<!>
}

// Match Go: a named object alone is not a class_declaration; its name does not justify widening the heuristic.
object PriceObject {
    val currency: java.util.Currency = java.util.Currency.getInstance(java.util.Locale.getDefault())
}
