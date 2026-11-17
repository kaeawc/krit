// Smoke: the deprecated text clipboard, the superclass of android.content.ClipboardManager.
package stubs

import android.content.Context

@Suppress("DEPRECATION")
fun smokeTextClipboard(context: Context): Boolean {
    val clipboard = context.getSystemService(Context.CLIPBOARD_SERVICE) as? android.text.ClipboardManager ?: return false
    if (clipboard.hasText()) clipboard.text = "copied"
    return clipboard.hasText()
}
