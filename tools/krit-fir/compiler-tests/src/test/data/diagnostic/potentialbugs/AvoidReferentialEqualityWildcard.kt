// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
package wildcardlookalike

import referentiallookalike.*

// The default kotlin.String wins over this wildcard-imported lookalike in both engines.
// Accessing length also checks that Kotlin did not bind the shared stub's empty class.
fun length(text: String) = text.length
fun same(a: String, candidate: Any) = <!AvoidReferentialEquality!>a === candidate<!>
