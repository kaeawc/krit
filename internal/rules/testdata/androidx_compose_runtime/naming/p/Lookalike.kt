package p.other

annotation class Composable

@Composable
fun lowercase() {}

fun compositionLocalOf(f: () -> Int) = f()
val NotALocal = compositionLocalOf { 1 }
