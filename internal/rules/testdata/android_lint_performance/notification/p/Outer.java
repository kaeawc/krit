package p;

import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.Intent;

public class Outer {
    public static class Inner extends BroadcastReceiver {
        @Override
        public void onReceive(Context context, Intent intent) {
            context.startActivity(intent);
            context.startActivities(new Intent[] { intent });
        }
    }
}
