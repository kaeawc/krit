package com.example;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;

public class ReplyNotification {
    PendingIntent explicit(Context context) {
        Intent intent = new Intent("com.example.REPLY");
        intent.setClassName(context, "com.example.ReplyActivity");
        return PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_MUTABLE);
    }

    PendingIntent escaped(Context context) {
        Intent intent = new Intent("com.example.REPLY");
        decorate(intent);
        return PendingIntent.getActivity(context, 0, intent, PendingIntent.FLAG_MUTABLE);
    }

    void decorate(Intent intent) {}
}
