// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 20
// Local lookalikes: classes in this package named like the Throwable base
// types shadow the default imports, so these objects extend plain classes.
package test

open class Error(val code: Int)

open class Exception

// Not reported, in Go or here: test.Error is not a Throwable.
object AppError : Error(42)

// Not reported, in Go or here: test.Exception is not a Throwable.
object AppException : Exception()

// A qualified reference still reaches the real Throwable, like Go.
object <!ObjectExtendsThrowable!>RealError<!> : kotlin.Error()

object <!ObjectExtendsThrowable!>RealException<!> : java.lang.Exception()
