// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 8
// An import alias of the constant. Go reports the import line, where the
// name MODE_WORLD_WRITEABLE appears, and so does FIR.
package test

import android.content.Context
<!WorldWriteableFiles!>import android.content.Context.MODE_WORLD_WRITEABLE as SHARED_WRITE<!>
import java.io.FileOutputStream

class AliasedFiles(private val context: Context) {
    // Deliberate improvement: Go misses this read, because the alias does not
    // spell MODE_WORLD_WRITEABLE; FIR resolves it to the Android constant, so
    // the file is opened world-writeable as the message says.
    fun open(): FileOutputStream = context.openFileOutput("a.txt", <!WorldWriteableFiles!>SHARED_WRITE<!>)

    fun private(): FileOutputStream = context.openFileOutput("b.txt", Context.MODE_PRIVATE)
}
