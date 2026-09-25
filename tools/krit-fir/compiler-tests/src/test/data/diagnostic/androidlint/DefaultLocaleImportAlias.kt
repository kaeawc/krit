// RENDER_DIAGNOSTICS_FULL_TEXT
// A true positive Go misses: the stdlib String.format spelled through an
// import alias. Go needs the call name `format`. (The alias hides the name
// `format` in this file, so it lives apart from DefaultLocaleRecall.kt.)
package test

import kotlin.text.format as fmt

fun aliased(value: Int): String = <!DefaultLocale!>String.fmt("%d", value)<!>
