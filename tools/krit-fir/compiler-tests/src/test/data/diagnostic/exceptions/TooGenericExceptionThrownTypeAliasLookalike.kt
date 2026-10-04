// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 15
// Same-file type aliases named like the generic exceptions, for other classes.
package test

class MyError(message: String) : IllegalStateException(message)

typealias Error = MyError
typealias Exception = IllegalArgumentException

// Divergence (precision): Go skips a name the file declares a class or object
// for, but not a type alias, so it resolves these names to java.lang and
// reports both throws. They construct MyError and IllegalArgumentException.
fun aliasOfProjectClass(): Nothing = throw Error("alias of a project class")
fun aliasOfSpecificClass(): Nothing = throw Exception("alias of a specific class")
