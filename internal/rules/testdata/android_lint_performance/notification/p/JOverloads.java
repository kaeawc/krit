package p;

import android.app.PendingIntent;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import androidx.core.app.NotificationCompat;

class JOverloads {
    void overloaded(Context context) {
        new NotificationCompat.Builder(context, "c").setContentIntent(helper(context));
    }
    PendingIntent helper(Context context) {
        Intent intent = new Intent(context, JQuiet.class);
        return PendingIntent.getBroadcast(context, 0, intent, 0);
    }
    PendingIntent helper(String unused) {
        Intent intent = new Intent((Context) null, JReceiver.class);
        return PendingIntent.getBroadcast(null, 0, intent, 0);
    }

    static class Nested extends BroadcastReceiver {
        @Override
        public void onReceive(Context context, Intent intent) {
            context.startActivity(intent);
        }
    }

    void nested(Context context) {
        Intent intent = new Intent(context, Nested.class);
        PendingIntent pending = PendingIntent.getBroadcast(context, 0, intent, 0);
        new NotificationCompat.Builder(context, "c").setContentIntent(pending);
    }
}
