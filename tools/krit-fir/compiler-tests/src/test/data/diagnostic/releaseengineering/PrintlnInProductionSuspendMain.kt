// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: a suspend top-level main is the entry point too; Go matches any
// top-level function named main.
package test

suspend fun main() {
    println("suspend entry point")
}
