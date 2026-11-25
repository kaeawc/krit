package q;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

public class OtherReceiver extends BroadcastReceiver {
    @Override
    public void onReceive(Context context, Intent intent) {
        if (intent != null) {
            Runnable r = () -> context.startActivity(intent);
        }
    }
}
