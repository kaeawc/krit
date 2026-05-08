package test;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;
import android.content.IntentFilter;
import androidx.core.content.ContextCompat;

class ReceiverSetup {
    void setup(Context context, BroadcastReceiver receiver) {
        context.registerReceiver(receiver, new IntentFilter(Intent.ACTION_SCREEN_ON), Context.RECEIVER_NOT_EXPORTED);
        ContextCompat.registerReceiver(context, receiver, new IntentFilter(Intent.ACTION_USER_PRESENT), ContextCompat.RECEIVER_EXPORTED);
    }
}
