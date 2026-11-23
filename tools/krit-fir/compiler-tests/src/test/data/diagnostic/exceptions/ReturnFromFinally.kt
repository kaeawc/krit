// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 22, 33x2, 44, 55, 65, 75
package test

fun fetch(): String = "data"

fun returnInFinally(): String {
    var result = ""
    try {
        result = fetch()
    } catch (e: Exception) {
        result = "fallback"
    } finally {
        <!ReturnFromFinally!>return result<!>
    }
}

fun returnInFinallyOfTryExpression(): Int {
    val value = try {
        fetch().length
    } finally {
        <!ReturnFromFinally!>return 0<!>
    }
}

fun nestedFinally(): Int {
    try {
        fetch()
    } finally {
        try {
            fetch()
        } finally {
            <!ReturnFromFinally!>return 1<!>
        }
    }
}

// A return in a branch of the finally block still leaves the function.
fun returnInBranch(flag: Boolean): Int {
    try {
        fetch()
    } finally {
        if (flag) {
            <!ReturnFromFinally!>return 1<!>
        }
    }
    return 0
}

// A non-local return from an inline lambda leaves the function too.
fun returnFromInlineLambda(items: List<String>): Int {
    try {
        fetch()
    } finally {
        items.forEach { if (it.isEmpty()) <!ReturnFromFinally!>return 1<!> }
    }
    return 0
}

// A labeled return to the function leaves the block.
fun labeledToFunction(): Int {
    try {
        fetch()
    } finally {
        <!ReturnFromFinally!>return@labeledToFunction 2<!>
    }
}

// A labeled return to the lambda that encloses the whole try also leaves the block.
fun labeledToEnclosingLambda(): Int {
    return run {
        try {
            fetch().length
        } finally {
            <!ReturnFromFinally!>return@run 3<!>
        }
    }
}
