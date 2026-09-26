// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Positives Go misses: backticks around an identifier do not change the name,
// so each call below resolves to kotlin.check with a single `!=` null
// comparison of a nullable value. Go compares the raw callee text, backticks
// included ("`check`"), against "check".
package test

fun backticked(x: Any?) {
    <!UseCheckNotNull!>`check`(x != null)<!>
}

fun backtickedQualified(x: Any?) {
    <!UseCheckNotNull!>kotlin.`check`(x != null)<!>
}
