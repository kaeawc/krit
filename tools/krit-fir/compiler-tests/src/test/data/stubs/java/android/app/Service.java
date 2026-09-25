// Compiler-test source stub; never packaged in the production artifact.
package android.app;

import android.content.ContextWrapper;
import android.content.Intent;
import android.os.IBinder;

public abstract class Service extends ContextWrapper {
    public static final int START_STICKY = 1;

    public static final int START_NOT_STICKY = 2;

    public static final int START_REDELIVER_INTENT = 3;

    public static final int STOP_FOREGROUND_REMOVE = 1;

    public Service() {
        super(null);
    }

    public void onCreate() {
        throw new RuntimeException("Stub!");
    }

    public int onStartCommand(Intent intent, int flags, int startId) {
        throw new RuntimeException("Stub!");
    }

    public abstract IBinder onBind(Intent intent);

    public boolean onUnbind(Intent intent) {
        throw new RuntimeException("Stub!");
    }

    public void onDestroy() {
        throw new RuntimeException("Stub!");
    }

    public final void startForeground(int id, Notification notification) {
        throw new RuntimeException("Stub!");
    }

    public final void stopForeground(int flags) {
        throw new RuntimeException("Stub!");
    }

    public final void stopSelf() {
        throw new RuntimeException("Stub!");
    }
}
