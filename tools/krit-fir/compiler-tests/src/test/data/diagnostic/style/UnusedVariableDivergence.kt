// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Shapes where Go and the checker disagree.
package test.unusedvariabledivergence

fun delegated() {
    // Go misses this: it reads a local with no `=` in its text as a declaration without an initializer.
    val <!UnusedVariable!>lazyUnused<!> by lazy { 3 }
    val lazyUsed by lazy { 4 }
    println(lazyUsed)
}
