package test;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;

class InsecurePendingIntent {
    void schedule(Context context, Intent intent) {
        PendingIntent.getService(context, 0, intent, PendingIntent.FLAG_UPDATE_CURRENT);
    }
}
