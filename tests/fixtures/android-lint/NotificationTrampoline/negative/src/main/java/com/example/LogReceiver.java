package com.example;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

public class LogReceiver extends BroadcastReceiver {
    @Override
    public void onReceive(Context context, Intent intent) {
        // Does not launch an activity, so it is not a trampoline.
        context.sendBroadcast(intent);
    }
}
