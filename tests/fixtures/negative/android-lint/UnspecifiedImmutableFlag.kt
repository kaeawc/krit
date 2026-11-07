package com.example

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

class Notifier {
    fun open(context: Context, intent: Intent): PendingIntent =
        PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
}
