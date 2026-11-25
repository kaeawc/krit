package p;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;
import androidx.core.app.NotificationCompat;

class JHelper {
    void viaHelper(Context context) {
        new NotificationCompat.Builder(context, "c").setContentIntent(helper(context));
    }
    PendingIntent helper(Context context) {
        Intent intent = new Intent(context, JReceiver.class);
        return PendingIntent.getBroadcast(context, 0, intent, PendingIntent.FLAG_IMMUTABLE);
    }
    void direct(Context context) {
        new NotificationCompat.Builder(context, "c")
            .setContentTitle("t")
            .setContentIntent(PendingIntent.getService(context, 0, new Intent(context, JService.class), 0));
    }
    void quiet(Context context) {
        Intent intent = new Intent(context, JQuiet.class);
        PendingIntent pending = PendingIntent.getBroadcast(context, 0, intent, 0);
        new NotificationCompat.Builder(context, "c").setContentIntent(pending);
    }
    void reassigned(Context context) {
        Intent intent = new Intent(context, JQuiet.class);
        intent = new Intent(context, JReceiver.class);
        PendingIntent pending = PendingIntent.getBroadcast(context, 0, intent, 0);
        new NotificationCompat.Builder(context, "c").setContentIntent(pending);
    }
}
