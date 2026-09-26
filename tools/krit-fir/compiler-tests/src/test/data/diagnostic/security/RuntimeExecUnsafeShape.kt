// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 24, 28, 32, 36, 40, 44, 48, 52, 56, 60, 66, 72, 76, 80, 84, 88, 92
// Positives and negatives for RuntimeExecUnsafeShape: Runtime.exec(String)
// with an interpolated or non-statically concatenated command, reported on
// the command argument like Go.
@file:Suppress("DEPRECATION")

package test

import java.io.File

const val BASE_DIR = "/data"

object Paths {
    const val TOOL_DIR = "/opt"
}

class Runner {
    fun interpolated(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"ls -la $userPath"<!>)
    }

    fun braced(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"ls -la ${userPath.trim()}"<!>)
    }

    fun rawString(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"""ls -la $userPath"""<!>)
    }

    fun mixedConstantInterpolation(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"$BASE_DIR/ls $userPath"<!>)
    }

    fun computed(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"cat " + userPath<!>)
    }

    fun computedCall(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"cat " + userPath.trim()<!>)
    }

    fun computedFirst(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>userPath + " --help"<!>)
    }

    fun computedFile(dir: File) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"ls " + dir<!>)
    }

    fun qualified(userPath: String) {
        java.lang.Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"cat " + userPath<!>)
    }

    fun parenthesizedArgument(userPath: String) {
        Runtime.getRuntime().exec((<!RuntimeExecUnsafeShape!>"cat " + userPath<!>))
    }

    fun safeCall(userPath: String) {
        Runtime.getRuntime()?.exec(<!RuntimeExecUnsafeShape!>"cat $userPath"<!>)
    }

    fun multiline(userPath: String) {
        Runtime.getRuntime()
            .exec(
                <!RuntimeExecUnsafeShape!>"cat "<!> +
                    userPath,
            )
    }

    fun interpolationInsideCall(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>listOf("ls", "$userPath").joinToString(" ")<!>)
    }

    fun indexed(commands: List<String>, i: Int) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>commands[i + 1]<!>)
    }

    fun inLambda(userPath: String) {
        listOf(1).forEach { Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"cat " + userPath + it<!>) }
    }

    val anonymous = object {
        fun run(userPath: String) = Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"cat $userPath"<!>)
    }

    companion object {
        fun inCompanion(userPath: String) = Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"cat " + userPath<!>)
    }
}

fun topLevel(userPath: String) = Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"rm -rf $userPath"<!>)

class SafeRunner {
    fun array(userPath: String) {
        Runtime.getRuntime().exec(arrayOf("ls", "-la", userPath))
    }

    fun literal() {
        Runtime.getRuntime().exec("ls -la")
    }

    fun literalConcat() {
        Runtime.getRuntime().exec("ls " + "-la")
    }

    fun constantConcat() {
        Runtime.getRuntime().exec("ls " + BASE_DIR)
        Runtime.getRuntime().exec("ls " + Paths.TOOL_DIR)
        Runtime.getRuntime().exec("ls " + null)
    }

    fun constantInterpolation() {
        Runtime.getRuntime().exec("ls $BASE_DIR")
        Runtime.getRuntime().exec("ls ${Paths.TOOL_DIR}")
        Runtime.getRuntime().exec(listOf("ls", "$BASE_DIR").joinToString(" "))
    }

    fun plainVariable(command: String) {
        Runtime.getRuntime().exec(command)
        Runtime.getRuntime().exec(command.trim())
        Runtime.getRuntime().exec(String.format("ls %s", command))
    }

    fun otherOverloads(userPath: String, dir: File) {
        Runtime.getRuntime().exec("ls $userPath", null)
        Runtime.getRuntime().exec("ls " + userPath, arrayOf("A=1"), dir)
    }

    fun processBuilder(userPath: String) {
        ProcessBuilder("ls", "-la", userPath).start()
    }
}
