// Compiler-test source stub; never packaged in the production artifact.
package android.content;

import android.content.pm.PackageManager;
import android.content.res.Resources;
import android.content.res.TypedArray;
import android.net.Uri;
import android.os.Looper;
import android.util.AttributeSet;
import java.io.FileInputStream;
import java.io.FileNotFoundException;
import java.io.FileOutputStream;

public abstract class Context {
    public static final int MODE_PRIVATE = 0;

    @Deprecated
    public static final int MODE_WORLD_READABLE = 1;

    @Deprecated
    public static final int MODE_WORLD_WRITEABLE = 2;

    public static final int RECEIVER_EXPORTED = 2;

    public static final int RECEIVER_NOT_EXPORTED = 4;

    public static final String ACTIVITY_SERVICE = "activity";

    public static final String ALARM_SERVICE = "alarm";

    public static final String BLUETOOTH_SERVICE = "bluetooth";

    public static final String CAMERA_SERVICE = "camera";

    public static final String CONNECTIVITY_SERVICE = "connectivity";

    public static final String LAYOUT_INFLATER_SERVICE = "layout_inflater";

    public static final String LOCATION_SERVICE = "location";

    public static final String NOTIFICATION_SERVICE = "notification";

    public static final String POWER_SERVICE = "power";

    public Context() {
    }

    public abstract Resources getResources();

    public abstract Resources.Theme getTheme();

    public abstract String getPackageName();

    public abstract PackageManager getPackageManager();

    public abstract ContentResolver getContentResolver();

    public abstract Context getApplicationContext();

    public abstract Looper getMainLooper();

    public final String getString(int resId) {
        throw new RuntimeException("Stub!");
    }

    public final String getString(int resId, Object... formatArgs) {
        throw new RuntimeException("Stub!");
    }

    public final CharSequence getText(int resId) {
        throw new RuntimeException("Stub!");
    }

    public final int getColor(int id) {
        throw new RuntimeException("Stub!");
    }

    public final TypedArray obtainStyledAttributes(int[] attrs) {
        throw new RuntimeException("Stub!");
    }

    public final TypedArray obtainStyledAttributes(AttributeSet set, int[] attrs) {
        throw new RuntimeException("Stub!");
    }

    public abstract SharedPreferences getSharedPreferences(String name, int mode);

    public abstract FileInputStream openFileInput(String name) throws FileNotFoundException;

    public abstract FileOutputStream openFileOutput(String name, int mode) throws FileNotFoundException;

    public abstract void startActivity(Intent intent);

    public abstract ComponentName startService(Intent service);

    public abstract void sendBroadcast(Intent intent);

    public abstract Intent registerReceiver(BroadcastReceiver receiver, IntentFilter filter);

    public abstract Intent registerReceiver(BroadcastReceiver receiver, IntentFilter filter, int flags);

    public abstract void unregisterReceiver(BroadcastReceiver receiver);

    public abstract void enforceCallingPermission(String permission, String message);

    public abstract int checkCallingOrSelfPermission(String permission);

    public abstract int checkSelfPermission(String permission);

    public abstract void grantUriPermission(String toPackage, Uri uri, int modeFlags);

    public abstract void revokeUriPermission(Uri uri, int modeFlags);

    public abstract Object getSystemService(String name);

    public final <T> T getSystemService(Class<T> serviceClass) {
        throw new RuntimeException("Stub!");
    }
}
