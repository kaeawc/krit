// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16
package pendingflag.opaque

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

class PendingFlagOpaque {
    private val memberFlags = 0
    private val computedFlags: Int get() = 0
    private var mutableFlags = 0

    fun pending(context: Context, intent: Intent, unknown: Int) {
        // Both engines follow the stable same-file member val initializer.
        PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>memberFlags<!>)
        PendingIntent.getActivity(context, 0, intent, computedFlags)
        mutableFlags = unknown
        PendingIntent.getActivity(context, 0, intent, mutableFlags)
        val delegatedFlags by lazy { 0 }
        PendingIntent.getActivity(context, 0, intent, delegatedFlags)
        PendingIntent.getActivity(context, 0, intent, unknown)
    }
}
