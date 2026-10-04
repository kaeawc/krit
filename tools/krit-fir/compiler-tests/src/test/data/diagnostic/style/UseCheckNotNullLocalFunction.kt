// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 12
// Lookalike: a `check` declared in the file's own package shadows the default
// import of kotlin.check, so these calls are not kotlin.check. Go matches the
// callee name alone and reports both; the checker does not.
package test

fun check(value: Boolean) {}

fun shadowed(x: Any?) {
    check(x != null)
    check(null != x)
}
