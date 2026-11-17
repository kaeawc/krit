// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10
// True positive: a same-package type alias named Boolean takes priority over
// the default import of kotlin.Boolean. Go reads same-file aliases first, so
// it reports this too.
package test

typealias Boolean = String

val <!NonBooleanPropertyPrefixedWithIs!>isAliasShadowed<!>: Boolean = "shadowed"
