// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 13
// Precision: a same-package `single()` extension on List shadows the stdlib
// one, so the terminal is not kotlin.collections.single and `.single { }`
// would call the stdlib function instead. Go matches the name and reports it.
package test

fun <T> List<T>.single(): T = get(0)

fun shadowed(list: List<Int>): Int = list.filter { it > 0 }.single()

// The stdlib first is still reported after it.
fun stillStdlib(list: List<Int>): Int = <!UnnecessaryFilter!>list<!>.filter { it > 0 }.first()
