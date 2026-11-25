package p;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import androidx.core.app.NotificationCompat;

class JNotify {
    void a(Context context) {
        Intent intent = new Intent(context, TrampolineReceiver.class);
        PendingIntent pending = PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE);
        new NotificationCompat.Builder(context, "c").setContentIntent(pending);
    }
    void b(Context context) {
        PendingIntent pending = PendingIntent.getService(context, 0, new Intent("x"), PendingIntent.FLAG_IMMUTABLE);
        NotificationCompat.Builder builder = new NotificationCompat.Builder(context, "c");
        builder.setContentIntent(pending);
    }
}
