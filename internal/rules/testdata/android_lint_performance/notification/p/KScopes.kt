package p

import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat

class KScopes(private val context: Context) {
    fun shadowed() {
        val intent = Intent(context, JQuiet::class.java)
        run {
            val intent = Intent(context, JReceiver::class.java)
            intent.toString()
        }
        val pending = PendingIntent.getBroadcast(context, 0, intent, 0)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }
}
