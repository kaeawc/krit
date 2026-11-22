// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17
package pendingflag.resolution

import android.app.PendingIntent as PlatformPending
import android.app.PendingIntent.getBroadcast as broadcastPending
import android.content.Context
import android.content.Intent

private object PendingFlagConstants {
    const val ZERO = 0
    const val IMMUTABLE = PlatformPending.FLAG_IMMUTABLE
}

fun pendingFlagResolved(context: Context, intent: Intent) {
    // Both engines recognize the receiver import alias and the zero flags value.
    PlatformPending.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>0<!>)
    // Resolved imported factory: the former FIR-only addition is dropped-by-confirm-only.
    broadcastPending(context, 0, intent, 0)
    // Resolved qualified const: the former FIR-only addition is dropped-by-confirm-only.
    android.app.PendingIntent.getActivity(context, 0, intent, PendingFlagConstants.ZERO)
    android.app.PendingIntent.getActivity(context, 0, intent, PendingFlagConstants.IMMUTABLE)
}
