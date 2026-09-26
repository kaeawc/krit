// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// True positives Go misses. Go proves the receiver is a Throwable only when its
// last identifier is the nearest enclosing catch's variable; each of these is
// printStackTrace() on a Throwable all the same.
package test

class AppFailure(message: String) : RuntimeException(message) {
    // Go misses this because `super` is not the caught variable.
    override fun printStackTrace() {
        println("app failure")
        <!PrintStackTrace!>super.printStackTrace()<!>
    }

    // Go misses this because an implicit receiver has no identifier to match.
    fun dump() {
        <!PrintStackTrace!>printStackTrace()<!>
    }

    // Go misses this because `this` is not the caught variable.
    fun dumpExplicit() {
        <!PrintStackTrace!>this.printStackTrace()<!>
    }
}

// Go misses this because the receiver is a parameter, not a caught variable.
fun parameter(error: Throwable) {
    <!PrintStackTrace!>error.printStackTrace()<!>
}

// Go misses this because the receiver is a local, not a caught variable.
fun local() {
    val failure = IllegalStateException("bad state")
    <!PrintStackTrace!>failure.printStackTrace()<!>
}

// Go misses this because a constructor call has no identifier to match.
fun constructed() {
    <!PrintStackTrace!>RuntimeException("here").printStackTrace()<!>
}

// Go misses this because the receiver's last identifier is `cause`.
fun cause(e: Exception) {
    <!PrintStackTrace!>e.cause?.printStackTrace()<!>
}

// Go misses this because the receiver is a call.
fun result(outcome: Result<Int>) {
    <!PrintStackTrace!>outcome.exceptionOrNull()?.printStackTrace()<!>
}

// Go misses these because the caught variable is only compared with the
// nearest enclosing catch, which is the inner one.
fun nestedCatch() {
    try {
        work()
    } catch (outer: Exception) {
        try {
            work()
        } catch (inner: Exception) {
            <!PrintStackTrace!>outer.printStackTrace()<!>
        }
    }
}

// Go misses this because a call on an implicit `with` receiver has no receiver
// identifier.
fun withReceiver() {
    try {
        work()
    } catch (e: Exception) {
        with(e) {
            <!PrintStackTrace!>printStackTrace()<!>
        }
    }
}

// Go misses this because the receiver's last identifier is `cause` while the
// caught variable is `e`.
fun caughtCause() {
    try {
        work()
    } catch (e: Exception) {
        <!PrintStackTrace!>e.cause?.printStackTrace()<!>
    }
}

fun work() {}

// Go misses this because the receiver is a parameter; a type parameter bounded
// by Throwable resolves to the stdlib extension.
fun <T : Throwable> bounded(failure: T) {
    <!PrintStackTrace!>failure.printStackTrace()<!>
}

// Members of an object expression and of a local class that extend Exception
// and override printStackTrace(): the owner comes from the symbol, with no
// class-id lookup. Go misses these because the receivers are not caught
// variables.
fun localOwners() {
    val anonymous = object : Exception("anonymous") {
        override fun printStackTrace() {
            <!PrintStackTrace!>super.printStackTrace()<!>
        }
    }
    <!PrintStackTrace!>anonymous.printStackTrace()<!>

    class LocalFailure : Exception() {
        override fun printStackTrace() {}
    }
    <!PrintStackTrace!>LocalFailure().printStackTrace()<!>
}
