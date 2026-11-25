package p

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat
import q.OtherReceiver

class KEdge(private val context: Context) {
    fun nested() {
        val intent = Intent(context, Outer.Inner::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, 0)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }
    fun imported(builder: NotificationCompat.Builder) {
        val intent = Intent(context, OtherReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, 0)
        with(builder) {
            setContentIntent(pending)
        }
    }
    fun foreground() {
        val intent = Intent(context, JService::class.java)
        val pending = PendingIntent.getForegroundService(context, 0, intent, 0)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }
    fun commented() {
        // NotificationCompat.Builder(context, "c").setContentIntent(PendingIntent.getBroadcast(context, 0, Intent("x"), 0))
        val s = "NotificationCompat.Builder(context, \"c\").setContentIntent(p)"
    }
}
