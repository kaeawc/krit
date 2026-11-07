package test;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;

class ReceiverSetup {
    void setup(Context context, BroadcastReceiver receiver) {
        context.registerReceiver(receiver, new IntentFilter("com.example.SYNC_DONE"));
        context.registerReceiver(receiver, new IntentFilter("com.example.REFRESH"), 0);
    }
}
