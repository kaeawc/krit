// Compiler-test source stub; never packaged in the production artifact.
package android.content;

import android.content.pm.PackageManager;
import android.content.res.Resources;
import android.os.Looper;

public class ContextWrapper extends Context {
    public ContextWrapper(Context base) {
    }

    protected void attachBaseContext(Context base) {
        throw new RuntimeException("Stub!");
    }

    public Context getBaseContext() {
        throw new RuntimeException("Stub!");
    }

    public Resources getResources() {
        throw new RuntimeException("Stub!");
    }

    public Resources.Theme getTheme() {
        throw new RuntimeException("Stub!");
    }

    public String getPackageName() {
        throw new RuntimeException("Stub!");
    }

    public PackageManager getPackageManager() {
        throw new RuntimeException("Stub!");
    }

    public ContentResolver getContentResolver() {
        throw new RuntimeException("Stub!");
    }

    public Context getApplicationContext() {
        throw new RuntimeException("Stub!");
    }

    public Looper getMainLooper() {
        throw new RuntimeException("Stub!");
    }

    public SharedPreferences getSharedPreferences(String name, int mode) {
        throw new RuntimeException("Stub!");
    }

    public void startActivity(Intent intent) {
        throw new RuntimeException("Stub!");
    }

    public ComponentName startService(Intent service) {
        throw new RuntimeException("Stub!");
    }

    public void sendBroadcast(Intent intent) {
        throw new RuntimeException("Stub!");
    }

    public Intent registerReceiver(BroadcastReceiver receiver, IntentFilter filter) {
        throw new RuntimeException("Stub!");
    }

    public Intent registerReceiver(BroadcastReceiver receiver, IntentFilter filter, int flags) {
        throw new RuntimeException("Stub!");
    }

    public void unregisterReceiver(BroadcastReceiver receiver) {
        throw new RuntimeException("Stub!");
    }

    public void enforceCallingPermission(String permission, String message) {
        throw new RuntimeException("Stub!");
    }

    public int checkCallingOrSelfPermission(String permission) {
        throw new RuntimeException("Stub!");
    }

    public int checkSelfPermission(String permission) {
        throw new RuntimeException("Stub!");
    }

    public Object getSystemService(String name) {
        throw new RuntimeException("Stub!");
    }
}
