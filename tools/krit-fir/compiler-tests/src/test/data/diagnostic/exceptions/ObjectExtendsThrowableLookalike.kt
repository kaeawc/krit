// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 17, 20, 22
// Local lookalikes: classes in this package named like the Throwable base
// types shadow the default imports, so these objects extend plain classes.
package test

open class Error(val code: Int)

open class Exception

// Go reports this because the supertype is named Error; FIR is correct to
// drop it because test.Error is not a Throwable.
object AppError : Error(42)

// Go reports this because the supertype is named Exception; FIR is correct to
// drop it because test.Exception is not a Throwable.
object AppException : Exception()

// A qualified reference still reaches the real Throwable, like Go.
<!ObjectExtendsThrowable!>object<!> RealError : kotlin.Error()

<!ObjectExtendsThrowable!>object<!> RealException : java.lang.Exception()
