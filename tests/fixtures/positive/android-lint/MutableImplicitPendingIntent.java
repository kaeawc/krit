package com.example;

import android.app.PendingIntent;
import android.content.Context;
import android.content.Intent;

public class ReplyNotification {
    PendingIntent replyAction(Context context) {
        return PendingIntent.getActivity(context, 0, new Intent("com.example.REPLY"), PendingIntent.FLAG_MUTABLE);
    }
}
