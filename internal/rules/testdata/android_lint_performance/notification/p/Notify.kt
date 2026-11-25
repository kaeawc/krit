package p

import android.app.Notification
import android.app.PendingIntent
import android.content.Context
import android.content.Intent
import androidx.core.app.NotificationCompat

class Notify(private val context: Context) {
    fun trampolineReceiver() {
        val intent = Intent(context, TrampolineReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "c").setContentIntent(pending).build()
    }

    fun trampolineService() {
        val intent = Intent(context, TrampolineService::class.java)
        val pending = PendingIntent.getService(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        val builder = NotificationCompat.Builder(context, "c")
        builder.setFullScreenIntent(pending, true)
    }

    fun quietReceiver() {
        val intent = Intent(context, QuietReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }

    fun actionTrampoline() {
        val intent = Intent(context, TrampolineReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "c").addAction(0, "x", pending)
    }

    fun actionQuiet() {
        val intent = Intent(context, QuietReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "c").addAction(0, "x", pending)
    }

    fun inlineIntent() {
        val pending = PendingIntent.getBroadcast(context, 0, Intent(context, TrampolineReceiver::class.java), PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }

    fun directCall() {
        NotificationCompat.Builder(context, "c").setContentIntent(
            PendingIntent.getBroadcast(context, 0, Intent("action"), PendingIntent.FLAG_IMMUTABLE))
    }

    fun actionIntent() {
        val intent = Intent("some.action")
        val pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }

    fun viaHelper() {
        NotificationCompat.Builder(context, "c").setContentIntent(helper())
    }

    private fun helper(): PendingIntent {
        val intent = Intent(context, TrampolineReceiver::class.java)
        return PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
    }

    fun activity() {
        val intent = Intent(context, MainActivity::class.java)
        val pending = PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        NotificationCompat.Builder(context, "c").setContentIntent(pending)
    }

    fun platformBuilder() {
        val intent = Intent(context, TrampolineReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        Notification.Builder(context, "c").setContentIntent(pending)
    }

    fun javaClassRef() {
        val intent = Intent(context, TrampolineReceiver::class.java)
        val pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        val b: NotificationCompat.Builder = NotificationCompat.Builder(context, "c")
        b.setContentIntent(pending)
    }
}
