// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// Negatives that match Go: the flags carry a mutability flag, or are not visible.
package test.implicitpendingintentnegative

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

fun mutabilityFlag(context: Context, intent: Intent) {
    PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
    PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_MUTABLE)
    PendingIntent.getService(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
    PendingIntent.getActivities(context, 0, arrayOf(intent), PendingIntent.FLAG_IMMUTABLE or PendingIntent.FLAG_ONE_SHOT)
}

fun multiLineMutability(context: Context, intent: Intent) {
    PendingIntent.getBroadcast(
        context,
        0,
        intent,
        PendingIntent.FLAG_UPDATE_CURRENT or
            PendingIntent.FLAG_IMMUTABLE,
    )
}

fun opaqueFlags(context: Context, intent: Intent, flags: Int) {
    PendingIntent.getActivity(context, 0, intent, flags)
    val local = PendingIntent.FLAG_UPDATE_CURRENT
    PendingIntent.getService(context, 0, intent, local)
    PendingIntent.getBroadcast(context, 0, intent, helperFlags())
    PendingIntent.getBroadcast(context, 0, intent, (flags))
}

fun helperFlags(): Int = PendingIntent.FLAG_MUTABLE

fun conditionalFlags(context: Context, intent: Intent, sdk: Int) {
    val flags = if (sdk >= 31) PendingIntent.FLAG_IMMUTABLE else 0
    PendingIntent.getActivity(context, 0, intent, flags)
    PendingIntent.getActivity(
        context,
        0,
        intent,
        PendingIntent.FLAG_UPDATE_CURRENT or if (sdk >= 31) PendingIntent.FLAG_IMMUTABLE else 0,
    )
}

fun otherMembers(pending: PendingIntent) {
    pending.cancel()
}
