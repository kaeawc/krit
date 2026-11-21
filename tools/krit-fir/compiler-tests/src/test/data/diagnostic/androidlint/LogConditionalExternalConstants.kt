// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Go is silent without a message call, regardless of declarations; FIR/Lint resolve these consts.
package test.logconditional.constants

import android.util.Log
import test.logconditional.external.Config
import test.logconditional.external.Config.prefix
import test.logconditional.external.Config.timeoutMs

private fun compute(): String = "computed"
typealias Sink = (prefix: String) -> Unit
class CallbackHolder {
    val callback: (prefix: String) -> Unit = { println(it) }
}
fun unrelated() { val prefix = compute(); val timeoutMs = compute() }
fun constants() {
    Log.d("Tag", "timeout=${Config.timeoutMs}")
    Log.d("Tag", "t" + Config.timeoutMs)
    Log.d("Tag", "timeout=$timeoutMs")
    Log.d("Tag", "t" + timeoutMs)
    Log.d("Tag", "${prefix}constant")
}
