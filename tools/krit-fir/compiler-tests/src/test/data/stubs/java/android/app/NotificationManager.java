// Compiler-test source stub; never packaged in the production artifact.
package android.app;

public class NotificationManager {
    public static final int IMPORTANCE_DEFAULT = 3;

    public static final int IMPORTANCE_HIGH = 4;

    NotificationManager() {
    }

    public void notify(int id, Notification notification) {
        throw new RuntimeException("Stub!");
    }

    public void notify(String tag, int id, Notification notification) {
        throw new RuntimeException("Stub!");
    }

    public void cancel(int id) {
        throw new RuntimeException("Stub!");
    }

    public void cancelAll() {
        throw new RuntimeException("Stub!");
    }

    public boolean areNotificationsEnabled() {
        throw new RuntimeException("Stub!");
    }
}
