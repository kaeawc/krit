// RENDER_DIAGNOSTICS_FULL_TEXT
// A project class that reuses the name of a well-known exception on the
// parent side of the comparison. Go looks the written simple names up in its
// table of well-known exceptions, so it reads this Exception as
// java.lang.Exception. FIR resolves the project class.
package shadowedparent

import java.io.IOException

// Shadows kotlin.Exception / java.lang.Exception for this package.
class Exception : Throwable()

fun risky() {
    throw IOException("fail")
}

// Go reports "Catch block for 'IOException' is unreachable because 'Exception'
// is caught above." That is false: this Exception is the project class above,
// which extends Throwable, so it does not catch an IOException. FIR is correct
// to drop it.
fun projectParentLookalike() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } catch (e: IOException) {
        println(e)
    }
}

// The project Exception still catches itself: a duplicate is reported, as Go
// does.
fun projectParentDuplicate() {
    try {
        risky()
    } catch (e: Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: Exception) {
        println(e)
    }
}

// The real exception, written fully qualified, still shadows IOException. Go
// misses this one (its table knows only simple names); FIR reports it.
fun qualifiedRealParent() {
    try {
        risky()
    } catch (e: java.lang.Exception) {
        println(e)
    } <!UnreachableCatchBlock!>catch<!> (e: IOException) {
        println(e)
    }
}
