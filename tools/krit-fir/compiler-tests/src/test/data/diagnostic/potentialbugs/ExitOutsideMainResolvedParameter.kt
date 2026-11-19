// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 20
// Negatives Go and detekt report: both compare the written parameter type with
// `Array<String>`, so a qualified `Array<kotlin.String>` or a type alias of it
// does not count for them. The parameter type is the same `Array<String>`, so
// this is a main function and the call is inside main.
package test

import kotlin.system.exitProcess

typealias Args = Array<String>

fun main(args: Array<kotlin.String>) {
    exitProcess(args.size)
}

object Tool {
    @JvmStatic
    fun main(args: Args) {
        exitProcess(args.size)
    }
}
