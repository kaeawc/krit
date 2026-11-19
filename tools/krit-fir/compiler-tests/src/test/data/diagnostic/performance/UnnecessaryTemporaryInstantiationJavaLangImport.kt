// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9
// Positive: an explicit import of java.lang.Long makes the simple name the
// wrapper, so `Long(x)` constructs one. Go and detekt report it too.
package test

import java.lang.Long

fun imported(x: kotlin.Long): String = <!UnnecessaryTemporaryInstantiation!>Long(x)<!>.toString()
