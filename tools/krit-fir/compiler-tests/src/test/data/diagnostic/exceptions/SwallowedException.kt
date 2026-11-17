// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 22, 30, 38, 46, 54, 62, 71, 79, 87, 99, 108
package test

fun doWork() {}

class Wrapped(message: String?) : RuntimeException(message)

class WrappedCause(message: String?, cause: Throwable?) : RuntimeException(message, cause)

fun neverUsed() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: Exception) {
        println("error occurred")
    }
}

fun commentOnly() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: Exception) {
        // e is mentioned only here
    }
}

fun thrownMessage() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: IllegalStateException) {
        throw IllegalArgumentException(e.message)
    }
}

fun thrownToString() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: IllegalStateException) {
        throw IllegalArgumentException(e.toString())
    }
}

fun thrownTemplate() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: IllegalStateException) {
        throw IllegalArgumentException("failed: ${e.message}")
    }
}

fun thrownSafeChain() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: IllegalStateException) {
        throw Wrapped(e.cause?.message)
    }
}

fun thrownThroughLocal() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: IllegalStateException) {
        val reason = e.message
        throw Wrapped(reason)
    }
}

fun thrownNotNullChain() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: IllegalStateException) {
        throw Wrapped(e.cause!!.message)
    }
}

fun thrownInsideLambda() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: IllegalStateException) {
        run { throw Wrapped(e.message) }
    }
}

fun qualifiedTypeName() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: java.io.IOException) {
        println("io")
    }
}

fun nestedTry() {
    try {
        doWork()
    } catch (outer: Exception) {
        println(outer)
        try {
            doWork()
        } <!SwallowedException!>catch<!> (inner: Exception) {
            println("inner failed")
        }
    }
}

fun smartCastMember() {
    try {
        doWork()
    } <!SwallowedException!>catch<!> (e: Exception) {
        if (e is IllegalStateException) {
            throw Wrapped(e.message)
        }
    }
}
