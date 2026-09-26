// Compiler-test source stub; never packaged in the production artifact.
package android.os;

public final class PowerManager {
    public static final int PARTIAL_WAKE_LOCK = 1;

    public static final int RELEASE_FLAG_WAIT_FOR_NO_PROXIMITY = 1;

    PowerManager() {
    }

    public boolean isInteractive() {
        throw new RuntimeException("Stub!");
    }

    public WakeLock newWakeLock(int levelAndFlags, String tag) {
        throw new RuntimeException("Stub!");
    }

    public boolean isIgnoringBatteryOptimizations(String packageName) {
        throw new RuntimeException("Stub!");
    }

    public final class WakeLock {
        WakeLock() {
        }

        public boolean isHeld() {
            throw new RuntimeException("Stub!");
        }

        public void acquire() {
            throw new RuntimeException("Stub!");
        }

        public void acquire(long timeout) {
            throw new RuntimeException("Stub!");
        }

        public void release() {
            throw new RuntimeException("Stub!");
        }

        public void release(int flags) {
            throw new RuntimeException("Stub!");
        }
    }
}
