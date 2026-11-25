package com.example

import android.app.PendingIntent
import android.content.Context
import android.content.Intent

class ReplyNotification(private val context: Context) {
    fun replyAction(): PendingIntent {
        val intent = Intent("com.example.REPLY")
        intent.putExtra("thread", 42)
        return PendingIntent.getBroadcast(
            context,
            0,
            intent,
            PendingIntent.FLAG_MUTABLE or PendingIntent.FLAG_UPDATE_CURRENT,
        )
    }
}
