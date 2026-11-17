// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14
// Types that only share a name with the coroutines ones.
package test

class MyFlow<T>(val value: T)

// Not a kotlinx Flow.
suspend fun ownType(): MyFlow<Int> = MyFlow(1)

// Go reads the written name, so a project type called Flow looks like the coroutines one.
interface Flow<T>

suspend fun sameName(): Flow<Int> = object : Flow<Int> {}
