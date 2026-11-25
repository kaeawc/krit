package com.example

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

class ReplyActivity

class ReplyNotification(private val context: Context) {
    fun explicit(): PendingIntent =
        PendingIntent.getActivity(context, 0, Intent(context, ReplyActivity::class.java), PendingIntent.FLAG_MUTABLE)

    fun withPackage(): PendingIntent {
        val intent = Intent("com.example.REPLY").setPackage(context.packageName)
        return PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_MUTABLE)
    }

    fun immutable(): PendingIntent =
        PendingIntent.getBroadcast(context, 0, Intent("com.example.REPLY"), PendingIntent.FLAG_IMMUTABLE)

    fun existing(): PendingIntent? =
        PendingIntent.getBroadcast(
            context,
            0,
            Intent("com.example.REPLY"),
            PendingIntent.FLAG_MUTABLE or PendingIntent.FLAG_NO_CREATE,
        )

    fun handedOff(intent: Intent): PendingIntent =
        PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_MUTABLE)
}
