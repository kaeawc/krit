package p

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat

class KNotify3(private val context: Context) {
    fun service() {
        val intent = Intent(context, JService::class.java)
        val pending = PendingIntent.getService(context, 0, intent, 0)
        val builder = NotificationCompat.Builder(context, "c")
        builder.setFullScreenIntent(pending, true)
        builder.addAction(0, "x", pending)
    }
    fun viaHelper() {
        NotificationCompat.Builder(context, "c").setContentIntent(helper())
    }
    private fun helper(): PendingIntent {
        val intent = Intent(context, JReceiver::class.java)
        return PendingIntent.getBroadcast(context, 0, intent, 0)
    }
    fun quiet() {
        val intent = Intent(context, JQuiet::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, 0)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }
    fun chained() {
        NotificationCompat.Builder(context, "c")
            .setContentTitle("t")
            .setContentIntent(PendingIntent.getBroadcast(context, 0, Intent(context, JReceiver::class.java), 0))
            .build()
    }
    fun withApply() {
        val intent = Intent(context, JReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, 0)
        NotificationCompat.Builder(context, "c").apply {
            setContentIntent(pending)
        }
    }
}
