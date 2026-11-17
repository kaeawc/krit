// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 14
// Same-file type aliases named like the generic exceptions, for other classes.
package test

class MyError(message: String) : IllegalStateException(message)

typealias Error = MyError
typealias Exception = IllegalArgumentException

// Divergence (precision): Go, like detekt, matches the written name and
// reports both throws. They construct MyError and IllegalArgumentException.
fun aliasOfProjectClass(): Nothing = throw Error("alias of a project class")
fun aliasOfSpecificClass(): Nothing = throw Exception("alias of a specific class")
