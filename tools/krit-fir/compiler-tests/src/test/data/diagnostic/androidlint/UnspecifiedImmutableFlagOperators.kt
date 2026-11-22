// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 15, 16, 17
package pendingflag.operators

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

fun pendingFlagOperators(context: Context, intent: Intent) {
    // Resolved Int.plus proves zero; the former FIR-only addition is dropped-by-confirm-only.
    PendingIntent.getActivity(context, 0, intent, 0.plus(0))
    // Resolved Int operators prove zero; the former FIR-only addition is dropped-by-confirm-only.
    PendingIntent.getActivity(context, 0, intent, 1.inv().and(0))
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>(8 / 2) % 4<!>)
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>8 shr 3<!>)
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>8 ushr 3<!>)
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>+0<!>)
    PendingIntent.getActivity(context, 0, intent, -1)
}
