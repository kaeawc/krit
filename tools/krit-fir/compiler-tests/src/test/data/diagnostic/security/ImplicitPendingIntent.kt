// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 12, 13, 14, 15, 19, 20, 24, 28, 32, 42, 46, 49
// Positives that match Go: a PendingIntent factory whose flags are visible and hold no
// FLAG_IMMUTABLE or FLAG_MUTABLE.
package test.implicitpendingintent

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

fun factories(context: Context, intent: Intent) {
    <!ImplicitPendingIntent!>PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)<!>
    <!ImplicitPendingIntent!>PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_ONE_SHOT)<!>
    <!ImplicitPendingIntent!>PendingIntent.getService(context, 0, intent, PendingIntent.FLAG_CANCEL_CURRENT)<!>
    <!ImplicitPendingIntent!>PendingIntent.getActivities(context, 0, arrayOf(intent), PendingIntent.FLAG_NO_CREATE)<!>
}

fun combined(context: Context, intent: Intent) {
    <!ImplicitPendingIntent!>PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_ONE_SHOT)<!>
    <!ImplicitPendingIntent!>PendingIntent.getActivity(context, 0, intent, (PendingIntent.FLAG_UPDATE_CURRENT))<!>
}

fun literal(context: Context, intent: Intent) {
    <!ImplicitPendingIntent!>PendingIntent.getBroadcast(context, 0, intent, 0)<!>
}

fun withOptions(context: Context, intent: Intent, options: android.os.Bundle) {
    <!ImplicitPendingIntent!>PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT, options)<!>
}

fun multiLine(context: Context, intent: Intent) {
    <!ImplicitPendingIntent!>PendingIntent<!>.getBroadcast(
        context,
        0,
        intent,
        PendingIntent.FLAG_UPDATE_CURRENT or
            PendingIntent.FLAG_CANCEL_CURRENT,
    )
}

fun qualified(context: Context, intent: Intent) {
    <!ImplicitPendingIntent!>android.app.PendingIntent.getService(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT)<!>
}

class Notifier(private val context: Context) {
    private val pending = <!ImplicitPendingIntent!>PendingIntent.getActivity(context, 1, Intent(), PendingIntent.FLAG_UPDATE_CURRENT)<!>

    fun inLambda(intent: Intent) {
        run { <!ImplicitPendingIntent!>PendingIntent.getService(context, 2, intent, PendingIntent.FLAG_UPDATE_CURRENT)<!> }
    }
}
