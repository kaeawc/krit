// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 18, 25, 31, 37, 42, 46, 51, 57
// Positives: a function named `main` that is not detekt's main function does
// not exempt the exit calls inside it. Go follows the same rule.
package test

import kotlin.system.exitProcess

class Launcher {
    // A member of a class, not an object.
    fun main(code: Int) {
        <!ExitOutsideMain!>java.lang.System.exit(code)<!>
    }

    companion object {
        // An object member needs @JvmStatic.
        fun main() {
            <!ExitOutsideMain!>kotlin.system.exitProcess(0)<!>
        }
    }

    object Named {
        @JvmStatic
        internal fun main() {
            <!ExitOutsideMain!>exitProcess(1)<!>
        }
    }

    // Not public.
    private fun main(args: Array<String>) {
        <!ExitOutsideMain!>exitProcess(2)<!>
    }
}

// An extension is not main.
fun String.main() {
    <!ExitOutsideMain!>exitProcess(length)<!>
}

// An unexpected parameter.
fun main(code: Int) {
    <!ExitOutsideMain!>exitProcess(code)<!>
}

internal fun main(args: Array<String>, count: Int) {
    <!ExitOutsideMain!>exitProcess(count)<!>
}

// `Array<out String>` is not the written `Array<String>`.
fun main(args: Array<out String>) {
    <!ExitOutsideMain!>exitProcess(args.size)<!>
}

fun outer() {
    // A local function named main is not a top-level main.
    fun main() {
        <!ExitOutsideMain!>exitProcess(4)<!>
    }
    main()
}
