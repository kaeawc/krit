package p;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import androidx.core.app.NotificationCompat;

class JNotify2 {
    void a(Context context) {
        Intent intent = new Intent(context, JReceiver.class);
        PendingIntent pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE);
        new NotificationCompat.Builder(context, "c").setContentIntent(pending);
    }
    void b(Context context) {
        Intent intent = new Intent(context, JService.class);
        PendingIntent pending = PendingIntent.getService(context, 0, intent, PendingIntent.FLAG_IMMUTABLE);
        new NotificationCompat.Builder(context, "c").addAction(0, "x", pending);
    }
}
