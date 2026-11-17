// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Lookalike: a `check` declared in the file's own package shadows the default
// import of kotlin.check, so these calls are not kotlin.check. Neither Go nor
// the checker reports them: Go skips a bare call in a file that declares a
// top-level `check`.
package test

fun check(value: Boolean) {}

fun shadowed(x: Any?) {
    check(x != null)
    check(null != x)
}
