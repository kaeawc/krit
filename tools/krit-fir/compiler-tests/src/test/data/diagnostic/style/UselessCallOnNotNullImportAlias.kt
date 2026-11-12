// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// An import alias of the stdlib function is the same function, which Go does not read by name.
package test

import kotlin.collections.listOfNotNull as nonNullList
import kotlin.text.orEmpty as emptyIfNull

fun aliased(a: String): List<String> = <!UselessCallOnNotNull!>nonNullList<!>(a)

fun aliasedReceiver(a: String): String = a.<!UselessCallOnNotNull!>emptyIfNull<!>()
