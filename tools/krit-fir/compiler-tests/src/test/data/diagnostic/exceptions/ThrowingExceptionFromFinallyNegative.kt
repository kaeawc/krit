// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

fun fetch(): String = "data"

fun cleanup() {}

fun throwOutsideFinally() {
    try {
        fetch()
    } catch (e: Exception) {
        throw IllegalStateException(e)
    } finally {
        cleanup()
    }
}

fun throwInTryBlock() {
    try {
        throw IllegalStateException()
    } finally {
        cleanup()
    }
}

fun throwInLocalFunction() {
    try {
        fetch()
    } finally {
        fun fail() {
            throw IllegalStateException()
        }
        cleanup()
    }
}

fun throwInAnonymousFunction() {
    try {
        fetch()
    } finally {
        val fail = fun() {
            throw IllegalStateException()
        }
        cleanup()
    }
}

fun throwInObjectMember() {
    try {
        fetch()
    } finally {
        val failing = object {
            fun fail() {
                throw IllegalStateException()
            }
        }
        cleanup()
    }
}

fun throwInLocalClassInitializer() {
    try {
        fetch()
    } finally {
        class Failing {
            init {
                throw IllegalStateException()
            }
        }
        cleanup()
    }
}

// Code in a try block or catch block of a try that sits in a function declared in the finally block.
fun throwInTryOfLocalFunction() {
    try {
        fetch()
    } finally {
        fun guarded() {
            try {
                fetch()
            } finally {
                cleanup()
            }
            throw IllegalStateException()
        }
    }
}
