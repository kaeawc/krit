// Compiler-test source stub; never packaged in the production artifact.
package android.app;

public class AlarmManager {
    public static final int RTC_WAKEUP = 0;

    public static final int RTC = 1;

    public static final int ELAPSED_REALTIME_WAKEUP = 2;

    public static final int ELAPSED_REALTIME = 3;

    public static final long INTERVAL_FIFTEEN_MINUTES = 900000L;

    public static final long INTERVAL_HALF_HOUR = 1800000L;

    public static final long INTERVAL_HOUR = 3600000L;

    public static final long INTERVAL_HALF_DAY = 43200000L;

    public static final long INTERVAL_DAY = 86400000L;

    AlarmManager() {
    }

    public void set(int type, long triggerAtMillis, PendingIntent operation) {
        throw new RuntimeException("Stub!");
    }

    public void setRepeating(int type, long triggerAtMillis, long intervalMillis, PendingIntent operation) {
        throw new RuntimeException("Stub!");
    }

    public void setInexactRepeating(int type, long triggerAtMillis, long intervalMillis, PendingIntent operation) {
        throw new RuntimeException("Stub!");
    }

    public void setWindow(int type, long windowStartMillis, long windowLengthMillis, PendingIntent operation) {
        throw new RuntimeException("Stub!");
    }

    public void setExact(int type, long triggerAtMillis, PendingIntent operation) {
        throw new RuntimeException("Stub!");
    }

    public void setAndAllowWhileIdle(int type, long triggerAtMillis, PendingIntent operation) {
        throw new RuntimeException("Stub!");
    }

    public void setExactAndAllowWhileIdle(int type, long triggerAtMillis, PendingIntent operation) {
        throw new RuntimeException("Stub!");
    }

    public void cancel(PendingIntent operation) {
        throw new RuntimeException("Stub!");
    }

    public boolean canScheduleExactAlarms() {
        throw new RuntimeException("Stub!");
    }
}
