// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8
// An import alias of kotlin.Pair still constructs kotlin.Pair, as in detekt.
package test

import kotlin.Pair as KPair

fun aliased() = <!PreferToOverPairSyntax!>KPair<!>(1, 2)
