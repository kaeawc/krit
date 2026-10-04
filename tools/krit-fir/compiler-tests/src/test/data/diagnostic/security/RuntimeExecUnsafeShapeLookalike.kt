// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Lookalikes: exec calls that are not java.lang.Runtime.exec(String).
package test

class Runtime {
    fun exec(command: String) {}

    companion object {
        fun getRuntime(): Runtime = Runtime()
    }
}

class Shell {
    fun exec(command: String) {}
}

fun exec(command: String) {}

class LocalRunner {
    fun run(value: String, shell: Shell) {
        Runtime.getRuntime().exec("local $value")
        Runtime.getRuntime().exec("local " + value)
        shell.exec("ls $value")
        exec("ls " + value)
    }
}
