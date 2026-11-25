package p

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat

class KNotify2(private val context: Context) {
    fun a() {
        val intent = Intent(context, JReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }
}
