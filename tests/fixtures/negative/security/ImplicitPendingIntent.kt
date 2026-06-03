package test

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

fun securePendingIntent(context: Context, intent: Intent) {
    PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE)
    PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_MUTABLE)
}

fun multiLineFlags(context: Context, intent: Intent) {
    // Flag argument spans multiple lines; the mutability flag is on a later line.
    PendingIntent.getBroadcast(
        context,
        0,
        intent,
        PendingIntent.FLAG_UPDATE_CURRENT or
            PendingIntent.FLAG_IMMUTABLE,
    )
}

fun helperProvidedFlags(context: Context, intent: Intent, flags: Int) {
    // Flags supplied indirectly via a parameter/val — cannot prove they lack mutability.
    PendingIntent.getActivity(context, 0, intent, flags)
    val resolved = PendingIntentFlags.mutable() or PendingIntentFlags.updateCurrent()
    PendingIntent.getService(context, 0, intent, resolved)
    PendingIntent.getActivities(context, 0, arrayOf(intent), PendingIntentFlags.mutable())
}

object PendingIntentFlags {
    fun mutable(): Int = PendingIntent.FLAG_MUTABLE
    fun updateCurrent(): Int = PendingIntent.FLAG_UPDATE_CURRENT
}
