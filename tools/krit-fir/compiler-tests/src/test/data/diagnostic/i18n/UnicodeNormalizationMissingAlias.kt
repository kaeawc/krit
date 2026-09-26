// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15
// Import aliases: Go and FIR both read the call's name as written.
package unmalias

import kotlin.text.contains as has
import unmalias.includes as contains

fun String.includes(query: String): Boolean = indexOf(query) >= 0

// The stdlib contains imported under another name is not written contains.
fun findAliased(title: String, query: String): Boolean = title.has(query)

// Another text function imported as contains is written contains.
fun searchIncludes(title: String, query: String): Boolean = <!UnicodeNormalizationMissing!>title.contains(query)<!>

// An invoked callable reference is not a call written contains: Go reads no
// name through the parentheses, and neither does FIR.
fun findInvokedReference(title: String, query: String): Boolean = (title::contains)(query)
