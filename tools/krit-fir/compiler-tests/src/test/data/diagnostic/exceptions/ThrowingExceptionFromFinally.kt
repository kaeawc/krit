// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 19, 28, 40x2, 50, 58, 70
package test

fun fetch(): String = "data"

fun throwInFinally() {
    try {
        fetch()
    } finally {
        <!ThrowingExceptionFromFinally!>throw IllegalStateException("cleanup failed")<!>
    }
}

fun rethrowInFinally(e: Exception) {
    try {
        fetch()
    } finally {
        <!ThrowingExceptionFromFinally!>throw e<!>
    }
}

fun throwInBranch(flag: Boolean) {
    try {
        fetch()
    } finally {
        if (flag) {
            <!ThrowingExceptionFromFinally!>throw IllegalStateException()<!>
        }
    }
}

fun throwInNestedFinally() {
    try {
        fetch()
    } finally {
        try {
            fetch()
        } finally {
            <!ThrowingExceptionFromFinally!>throw IllegalStateException()<!>
        }
    }
}

// An inline lambda runs as part of the block, so its throw replaces the exception.
fun throwInInlineLambda(items: List<String>) {
    try {
        fetch()
    } finally {
        items.forEach { if (it.isEmpty()) <!ThrowingExceptionFromFinally!>throw IllegalStateException()<!> }
    }
}

fun throwInElvis(value: String?) {
    try {
        fetch()
    } finally {
        val checked = value ?: <!ThrowingExceptionFromFinally!>throw IllegalStateException()<!>
        println(checked)
    }
}

fun throwInCatchOfTryInFinally() {
    try {
        fetch()
    } finally {
        try {
            fetch()
        } catch (e: Exception) {
            <!ThrowingExceptionFromFinally!>throw e<!>
        }
    }
}
