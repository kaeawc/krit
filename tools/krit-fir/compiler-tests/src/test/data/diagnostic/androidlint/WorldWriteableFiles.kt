// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 17, 20, 22, 24, 26, 28, 31x2, 34, 41, 43, 45, 47, 50, 54, 58, 62, 68
// Positive: every read of Android's Context.MODE_WORLD_WRITEABLE, however it
// is spelled or wherever it appears, and the static import of it. Go reports
// each identifier by name; FIR reports the same lines by resolving the read
// to the android.content.Context field.
package test

import android.app.Activity
import android.content.Context
<!WorldWriteableFiles!>import android.content.Context.MODE_WORLD_WRITEABLE<!>
import android.content.SharedPreferences
import java.io.FileOutputStream

annotation class FileMode(val value: Int)

val sharedMode = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>

class FileHelper(private val context: Context) {
    fun qualified(): FileOutputStream = context.openFileOutput("a.txt", Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

    fun imported(): FileOutputStream = context.openFileOutput("b.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

    fun throughSubclass(): FileOutputStream = context.openFileOutput("c.txt", Activity.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

    fun prefs(): SharedPreferences = context.getSharedPreferences("p", Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)

    fun combined(): Int = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> or Context.MODE_WORLD_READABLE

    // Two reads on one line: Go and FIR both report twice.
    fun twice(): Int = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> or <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>

    fun multiLine(): Int = Context
        .<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>

    // Deliberate improvement: Go misses the short template form (`$NAME`),
    // which its identifier dispatch does not see; it reports the braced form
    // on the next line. Both read the constant, like every other read here.
    fun template(): String = "mode=$<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>"

    fun bracedTemplate(): String = "mode=${Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>}"

    fun reference(): Int = (Context::<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>).get()

    fun defaulted(mode: Int = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>): Int = mode

    @FileMode(Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
    fun annotated() {}

    fun inLambda(): Int = run { Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> }

    fun whenBranch(private: Boolean): Int = when {
        private -> Context.MODE_PRIVATE
        else -> Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
    }

    val anonymous = object {
        fun open(c: Context): FileOutputStream = c.openFileOutput("d.txt", Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
    }

    companion object {
        const val DEFAULT_MODE = Context.<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
    }
}

// Inherited: an unqualified read inside a Context subclass.
class ExportActivity : Activity() {
    fun export(): FileOutputStream = openFileOutput("e.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>)
}
