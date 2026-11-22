// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 15, 16, 17, 18, 19, 25, 26, 27, 28, 30, 45
package pendingflag.basic

import android.app.PendingIntent
import android.app.PendingIntent.FLAG_IMMUTABLE
import android.content.Context
import android.content.Intent

private const val NO_FLAGS = 0
private const val SAFE_FLAGS = PendingIntent.FLAG_MUTABLE

fun pendingFlagBasic(context: Context, intent: Intent, unknown: Int, condition: Boolean) {
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>0<!>)
    PendingIntent.getActivities(context, 0, arrayOf(intent), <!UnspecifiedImmutableFlag!>0<!>)
    PendingIntent.getBroadcast(context, 0, intent, <!UnspecifiedImmutableFlag!>PendingIntent.FLAG_ONE_SHOT<!>)
    PendingIntent.getService(context, 0, intent, <!UnspecifiedImmutableFlag!>PendingIntent.FLAG_NO_CREATE<!>)
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>NO_FLAGS<!>, null)
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>PendingIntent.FLAG_CANCEL_CURRENT or 1<!>)
    PendingIntent.getActivity(context, 0, intent, FLAG_IMMUTABLE)
    PendingIntent.getActivity(context, 0, intent, SAFE_FLAGS)
    PendingIntent.getActivity(context, 0, intent, FLAG_IMMUTABLE or PendingIntent.FLAG_NO_CREATE)
    PendingIntent.getActivity(context, 0, intent, 1 shl 25)
    PendingIntent.getActivity(context, 0, intent, 1 shl 26)
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>1 shl 24<!>)
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>FLAG_IMMUTABLE xor FLAG_IMMUTABLE<!>)
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>FLAG_IMMUTABLE and 0<!>)
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>(2 + 3) * 4 - 20<!>)
    val localFlags = 0
    PendingIntent.getActivity(context, 0, intent, <!UnspecifiedImmutableFlag!>localFlags<!>)
    PendingIntent.getActivity(context, 0, intent, unknown)
    PendingIntent.getActivity(context, 0, intent, if (condition) FLAG_IMMUTABLE else 0)
    var changingFlags = 0
    changingFlags = unknown
    PendingIntent.getActivity(context, 0, intent, changingFlags)
    // Go deliberately excludes getForegroundService, despite the platform requirement.
    PendingIntent.getForegroundService(context, 0, intent, 0)
    // Preserve Go's mask: UPDATE_CURRENT alone suppresses this rule.
    PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)
    PendingIntent.getActivity(context, 0, intent, 1 shl 27)
    PendingIntent.getActivity(
        context,
        0,
        intent,
        <!UnspecifiedImmutableFlag!>0<!>,
    )
}
