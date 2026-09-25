// Compiler-test source stub; never packaged in the production artifact.
package android.app;

import android.content.Intent;
import android.os.IBinder;

@Deprecated
public abstract class IntentService extends Service {
    public IntentService(String name) {
    }

    public void setIntentRedelivery(boolean enabled) {
        throw new RuntimeException("Stub!");
    }

    public IBinder onBind(Intent intent) {
        throw new RuntimeException("Stub!");
    }

    protected abstract void onHandleIntent(Intent intent);
}
