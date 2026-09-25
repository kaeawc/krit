// RENDER_DIAGNOSTICS_FULL_TEXT
// Negative: a suspend top-level main is the entry point too; Go matches any
// top-level function named main.
package test

suspend fun main() {
    println("suspend entry point")
}
