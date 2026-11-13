// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 7, 11, 16, 20, 24, 26, 30, 34, 38, 43, 49, 54, 61, 68, 75, 82, 91, 98
// Positives: a function parameter that the body never reads.
package test.unusedparameter

class Widget {
    fun render(<!UnusedParameter!>unused<!>: Int) {
        println("render")
    }

    private fun secret(first: Int, <!UnusedParameter!>second<!>: Int): Int {
        return first
    }
}

fun plain(<!UnusedParameter!>unused<!>: Int) {
    println("hello")
}

fun String.extension(<!UnusedParameter!>count<!>: Int): String {
    return this
}

fun expressionBody(<!UnusedParameter!>x<!>: Int): Int = 42

fun withDefault(<!UnusedParameter!>x<!>: Int = 3) {
    println("default")
}

fun varargs(vararg <!UnusedParameter!>items<!>: String) {
    println("varargs")
}

fun <T> generic(<!UnusedParameter!>value<!>: T): String {
    return "generic"
}

suspend fun suspending(<!UnusedParameter!>id<!>: Long) {
    println("suspending")
}

fun local() {
    fun inner(<!UnusedParameter!>unusedInner<!>: Int) {
        println("inner")
    }
    inner(1)
}

fun substringCollision(<!UnusedParameter!>id<!>: String) {
    val guid = "x"
    println(guid)
}

fun commentsAndStringsDoNotCount(<!UnusedParameter!>id<!>: String) {
    // id
    println("id")
}

fun multiline(
    used: Int,
    <!UnusedParameter!>unusedSecond<!>: String,
) {
    println(used)
}

class Holder {
    companion object {
        fun create(<!UnusedParameter!>seed<!>: Int): Holder {
            return Holder()
        }
    }
}

object Registry {
    fun lookup(<!UnusedParameter!>key<!>: String): Int {
        return 0
    }
}

enum class Mode {
    A {
        fun describe(<!UnusedParameter!>label<!>: String): String {
            return "a"
        }
    },
}

fun inObjectExpression() {
    val listener = object : Runnable {
        override fun run() {}
        fun helper(<!UnusedParameter!>token<!>: Int) {
            println("helper")
        }
    }
    listener.run()
}

fun nestedShadowedNames(<!UnusedParameter!>other<!>: Int) {
    val other = 5
    println(other)
}
