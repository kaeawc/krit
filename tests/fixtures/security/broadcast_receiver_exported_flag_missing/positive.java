package test;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;

class ReceiverSetup {
    void setup(Context context, BroadcastReceiver receiver) {
        context.registerReceiver(receiver, new IntentFilter(Intent.ACTION_SCREEN_ON));
        context.registerReceiver(receiver, new IntentFilter(Intent.ACTION_USER_PRESENT), 0);
    }
}
