// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17
package pendingflag.lookalike

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

class PendingFlagLookalikes {
    private object PendingIntent {
        fun getActivity(context: Context, requestCode: Int, intent: Intent, flags: Int) = flags
        const val FLAG_IMMUTABLE = 0
    }

    fun pendingFlagLookalike(context: Context, intent: Intent) {
        // Go trusts the import despite the nested shadow; FIR proves this is not the Android API.
        PendingIntent.getActivity(context, 0, intent, 0)
        // FIR proves zero, but this former FIR-only addition is dropped-by-confirm-only.
        android.app.PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
    }
}
