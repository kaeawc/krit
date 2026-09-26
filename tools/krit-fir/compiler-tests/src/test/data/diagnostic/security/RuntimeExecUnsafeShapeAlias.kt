// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13
// An import alias of java.lang.Runtime: Go resolves the alias through the
// file's imports, and FIR reads the resolved call.
@file:Suppress("DEPRECATION")

package test

import java.lang.Runtime as Rt

class AliasRunner {
    fun importAlias(userPath: String) {
        Rt.getRuntime().exec(<!RuntimeExecUnsafeShape!>"cat " + userPath<!>)
        Rt.getRuntime().exec("ls -la")
    }
}
