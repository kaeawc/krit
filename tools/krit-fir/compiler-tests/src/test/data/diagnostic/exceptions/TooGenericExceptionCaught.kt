// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 10, 15, 20, 25, 30, 35, 40, 45, 56, 67, 70, 80, 107
package test

fun work() {}

fun everyDefaultType() {
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: Exception) {
        println(e)
    }
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: Throwable) {
        println(e)
    }
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: Error) {
        println(e)
    }
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: RuntimeException) {
        println(e)
    }
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: NullPointerException) {
        println(e)
    }
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: ArrayIndexOutOfBoundsException) {
        println(e)
    }
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: IndexOutOfBoundsException) {
        println(e)
    }
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>e<!>: IllegalMonitorStateException) {
        println(e)
    }
}

// Each catch clause is checked; the rule ignores what the body does with the exception.
fun severalClauses() {
    try {
        work()
    } catch (e: IllegalStateException) {
        println(e)
    } catch (<!TooGenericExceptionCaught!>e<!>: Exception) {
        throw e
    } finally {
        work()
    }
}

fun nestedInLambdaAndCatch() {
    val run = {
        try {
            work()
        } catch (<!TooGenericExceptionCaught!>failure<!>: Exception) {
            try {
                work()
            } catch (<!TooGenericExceptionCaught!>inner<!>: Throwable) {
                println(inner)
            }
        }
    }
    run()
}

fun tryAsExpression(): Int = try {
    1
} catch (<!TooGenericExceptionCaught!>e<!>: Exception) {
    2
}

// Names that match the allowed regex `_|(ignore|expected).*` are skipped.
fun allowedNames() {
    try {
        work()
    } catch (_: Exception) {
        println("ignored")
    }
    try {
        work()
    } catch (ignoredFailure: Throwable) {
        println("ignored")
    }
    try {
        work()
    } catch (expectedFailure: RuntimeException) {
        println("expected")
    }
}

// A name that only contains the allowed prefix is not allowed: the regex is a full match on its own alternatives.
fun notAllowedByPrefixInside() {
    try {
        work()
    } catch (<!TooGenericExceptionCaught!>myIgnored<!>: Exception) {
        println(myIgnored)
    }
}
