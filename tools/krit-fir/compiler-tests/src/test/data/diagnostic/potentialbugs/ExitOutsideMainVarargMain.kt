// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: `vararg String` is a main parameter list, like `Array<String>`.
package test

import kotlin.system.exitProcess

fun main(vararg args: String) {
    exitProcess(args.size)
}
