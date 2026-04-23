// Minimal stubs for androidx.compose.runtime — used by compiler-tests, not shipped.
package androidx.compose.runtime

fun <T> remember(calculation: () -> T): T = calculation()
fun <T> remember(key1: Any?, calculation: () -> T): T = calculation()
