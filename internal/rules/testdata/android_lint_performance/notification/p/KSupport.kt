package p

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import android.support.v4.app.NotificationCompat

class KSupport(private val context: Context) {
    fun support() {
        val pending = PendingIntent.getBroadcast(context, 0, Intent("x"), 0)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }
}
