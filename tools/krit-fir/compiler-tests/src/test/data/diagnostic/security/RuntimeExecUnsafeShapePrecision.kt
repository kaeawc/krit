// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 18, 22, 23, 27, 28, 32, 33, 34
// Go reports each call below, but none passes a computed String command to
// Runtime.exec(String).
// - typedArray and arrayPlus call exec(String[]): Go reads the array
//   argument's text and reports it because the text holds an interpolation
//   or a top-level `+`.
// - escapedDollar and literalDollar pass plain literals: Go reads any `$` in
//   a literal's text as non-static (an escaped `\$`, or a `$` that starts no
//   template, like `$5` or `$ `) and calls the command computed.
@file:Suppress("DEPRECATION")

package test

class Precision {
    fun typedArray(userPath: String) {
        Runtime.getRuntime().exec(listOf("ls", "$userPath").toTypedArray())
        Runtime.getRuntime().exec(arrayOf<String>("ls", "$userPath"))
    }

    fun arrayPlus(base: Array<String>, userPath: String) {
        Runtime.getRuntime().exec(base + userPath)
        Runtime.getRuntime().exec(base + "-la")
    }

    fun escapedDollar() {
        Runtime.getRuntime().exec("echo \$HOME")
        Runtime.getRuntime().exec("echo " + "\$HOME")
    }

    fun literalDollar() {
        Runtime.getRuntime().exec("price $5")
        Runtime.getRuntime().exec("a $ b")
        Runtime.getRuntime().exec("""cost $5""")
    }
}
