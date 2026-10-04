// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negative: private and world-readable modes, and the constant's name in
// comments, KDoc, and strings. Neither Go nor FIR reports any of these.
package test

import android.content.Context
import java.io.FileOutputStream

/**
 * Never pass [Context.MODE_WORLD_WRITEABLE] here.
 */
class SafeFiles(private val context: Context) {
    // MODE_WORLD_WRITEABLE must never be used.
    private val rationale = "Avoid MODE_WORLD_WRITEABLE and MODE_WORLD_WRITABLE"

    private val raw = """Context.MODE_WORLD_WRITEABLE"""

    fun open(): FileOutputStream = context.openFileOutput("data.txt", Context.MODE_PRIVATE)

    fun readable(): FileOutputStream = context.openFileOutput("r.txt", Context.MODE_WORLD_READABLE)

    fun describe(): String = rationale + raw
}
