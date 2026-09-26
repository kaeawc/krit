// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 15, 19, 20
// Go reports each call below, but none passes a computed String command to
// Runtime.exec(String). The first two call exec(String[]): Go reads the array
// argument's text and reports it because the text holds an interpolation.
// The last two pass plain literals: Go reads the escaped `\$` in the literal's
// text as non-static and calls the command computed.
@file:Suppress("DEPRECATION")

package test

class Precision {
    fun typedArray(userPath: String) {
        Runtime.getRuntime().exec(listOf("ls", "$userPath").toTypedArray())
        Runtime.getRuntime().exec(arrayOf<String>("ls", "$userPath"))
    }

    fun escapedDollar() {
        Runtime.getRuntime().exec("echo \$HOME")
        Runtime.getRuntime().exec("echo " + "\$HOME")
    }
}
