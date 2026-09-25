// RENDER_DIAGNOSTICS_FULL_TEXT
// True positives Go misses: the stdlib String.format and String.toLowerCase()
// spelled through import aliases. Go needs the call name `format` or
// `toLowerCase`. (An alias hides the original name in this file, so these
// live apart from DefaultLocaleRecall.kt.)
package test

import kotlin.text.format as fmt
import kotlin.text.toLowerCase as lower

fun aliased(value: Int): String = <!DefaultLocale!>String.fmt("%d", value)<!>

@Suppress("DEPRECATION_ERROR")
fun aliasedLower(s: String): String = <!DefaultLocale!>s.lower()<!>
