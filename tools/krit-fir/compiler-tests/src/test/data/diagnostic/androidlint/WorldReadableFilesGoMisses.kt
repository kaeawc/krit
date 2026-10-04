// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 18, 24, 30
// Divergence (recall): uses of the platform constant that Go misses, each a
// true positive FIR resolves. Go reports an identifier node only when its text
// is exactly MODE_WORLD_READABLE.
// - A backticked name: Go compares the node text, which includes the
//   backticks, so it reports neither the backticked import nor the backticked
//   qualified use. It still reports the bare use of the imported name.
// - A simple string template `"$MODE_WORLD_READABLE"`: Go does not report
//   the interpolated identifier, which is the platform constant inherited by
//   the Activity subclass.
// - An import alias through a Context subclass (`Service ... as SW`): Go
//   reports the import by the imported name but misses the aliased use, so
//   the line using both names gets one Go finding and two FIR findings.
package test

import android.app.Activity
import android.app.Service.<!WorldReadableFiles!>MODE_WORLD_READABLE<!> as SW
import android.content.Context
import android.content.Context.<!WorldReadableFiles!>`MODE_WORLD_READABLE`<!>

fun backticked() = Context.<!WorldReadableFiles!>`MODE_WORLD_READABLE`<!>

fun importedBackticked(context: Context) = context.getSharedPreferences("data", <!WorldReadableFiles!>MODE_WORLD_READABLE<!>)

class Main : Activity() {
    fun template() = "$<!WorldReadableFiles!>MODE_WORLD_READABLE<!>"
}

fun both(context: Context) = context.getSharedPreferences("data", <!WorldReadableFiles!>MODE_WORLD_READABLE<!> or <!WorldReadableFiles!>SW<!>)
