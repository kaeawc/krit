// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

fun fetch(): String = "data"

fun cleanup() {}

fun returnAfterFinally(): String {
    var result = ""
    try {
        result = fetch()
    } finally {
        cleanup()
    }
    return result
}

fun returnInTryAndCatch(): String {
    try {
        return fetch()
    } catch (e: Exception) {
        return "fallback"
    } finally {
        cleanup()
    }
}

// A return inside a function declared in the finally block only leaves that function.
fun localFunction(): Int {
    try {
        fetch()
    } finally {
        fun helper(): Int {
            return 1
        }
        helper()
    }
    return 0
}

fun anonymousFunction(): Int {
    try {
        fetch()
    } finally {
        val helper = fun(): Int {
            return 1
        }
        helper()
    }
    return 0
}

fun objectMember(): Int {
    try {
        fetch()
    } finally {
        val helper = object {
            fun value(): Int {
                return 1
            }
        }
        helper.value()
    }
    return 0
}

fun localClass(): Int {
    try {
        fetch()
    } finally {
        class Helper {
            fun value(): Int {
                return 1
            }
        }
        Helper().value()
    }
    return 0
}

// A labeled return to a lambda inside the block only leaves that lambda.
fun labeledToInnerLambda(items: List<String>): Int {
    try {
        fetch()
    } finally {
        items.forEach { if (it.isEmpty()) return@forEach }
    }
    return 0
}
