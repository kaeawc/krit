// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Deliberate improvements: each call below passes an interpolated or computed
// command to java.lang.Runtime.exec(String), so each is a real finding. Go
// misses them because it needs the receiver to be spelled
// `Runtime.getRuntime()` (or `java.lang.Runtime.getRuntime()`): it cannot see a
// stored Runtime, an implicit `with` or extension receiver, a parenthesized
// or `!!` receiver, a typealias, or a statically imported getRuntime. It also
// reads a parenthesized operand starting with a literal as static text, and
// treats an argument whose interpolations all name constants as static even
// when it concatenates a non-static operand. FIR reads the resolved call.
@file:Suppress("DEPRECATION")

package test

import java.lang.Runtime.getRuntime

const val TOOL_DIR = "/opt"

typealias JvmRuntime = java.lang.Runtime

class GoMisses {
    private val runtime = Runtime.getRuntime()

    fun stored(userPath: String) {
        val rt = Runtime.getRuntime()
        rt.exec(<!RuntimeExecUnsafeShape!>"ls $userPath"<!>)
        runtime.exec(<!RuntimeExecUnsafeShape!>"ls " + userPath<!>)
    }

    fun implicitReceiver(userPath: String) {
        with(Runtime.getRuntime()) {
            exec(<!RuntimeExecUnsafeShape!>"ls $userPath"<!>)
        }
    }

    fun Runtime.extensionReceiver(userPath: String) = exec(<!RuntimeExecUnsafeShape!>"ls " + userPath<!>)

    fun parenthesizedReceiver(userPath: String) {
        (Runtime.getRuntime()).exec(<!RuntimeExecUnsafeShape!>"ls $userPath"<!>)
    }

    fun typeAlias(userPath: String) {
        JvmRuntime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"ls $userPath"<!>)
    }

    fun staticImport(userPath: String) {
        getRuntime().exec(<!RuntimeExecUnsafeShape!>"ls $userPath"<!>)
    }

    fun parenthesizedOperand(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"ls " + ("-la " + userPath)<!>)
    }

    fun constantInterpolationPlusComputed(userPath: String) {
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"$TOOL_DIR/ls " + userPath<!>)
        Runtime.getRuntime().exec(<!RuntimeExecUnsafeShape!>"$TOOL_DIR/ls " + "a".plus(userPath)<!>)
    }

    // Go needs the receiver to be the call `Runtime.getRuntime()`; here it is
    // the `!!` postfix expression around it.
    fun notNullAssertedReceiver(userPath: String) {
        Runtime.getRuntime()!!.exec(<!RuntimeExecUnsafeShape!>"ls $userPath"<!>)
    }
}
