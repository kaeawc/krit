package com.example

import android.content.Context
import android.widget.RemoteViews
import androidx.core.app.NotificationCompat

class NotificationBuilderUsage(private val context: Context) {
    fun buildNotification() {
        // NotificationCompat.Builder is NOT a View — setText here is a
        // builder method, not TextView#setText, and must not fire.
        NotificationCompat.Builder(context, "channel")
            .setContentTitle("hi")
            .setContentText("body")
            .setSubText("foo")

        // Generic builder chain rooted at NotificationCompat must be skipped.
        val builder = NotificationCompat.Builder(context, "ch")
        builder.setContentText("subtitle")
    }
}

class RemoteViewsUsage {
    fun render(remote: RemoteViews) {
        // RemoteViews.setText(int, CharSequence) is a remote IPC method,
        // NOT TextView#setText — must not fire.
        remote.setTextViewText(1, "label")
    }
}

class CustomBuilderApi {
    class Builder {
        fun setText(value: String): Builder = this
    }

    fun configure() {
        // Bare setText inside an unrelated class — no TextView evidence,
        // must not fire.
        val b = Builder()
        b.setText("foo")
    }
}

class PlainHolder {
    fun setText(value: String) {
        // Method named setText on a non-TextView class. Bare call has
        // no receiver evidence, must not fire.
        setText("recurse")
    }
}
