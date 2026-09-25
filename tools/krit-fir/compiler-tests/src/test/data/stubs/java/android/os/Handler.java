// Compiler-test source stub; never packaged in the production artifact.
package android.os;

public class Handler {
    @Deprecated
    public Handler() {
    }

    public Handler(Looper looper) {
    }

    public Handler(Looper looper, Callback callback) {
    }

    public final boolean post(Runnable r) {
        throw new RuntimeException("Stub!");
    }

    public final boolean postDelayed(Runnable r, long delayMillis) {
        throw new RuntimeException("Stub!");
    }

    public final void removeCallbacks(Runnable r) {
        throw new RuntimeException("Stub!");
    }

    public final void removeCallbacksAndMessages(Object token) {
        throw new RuntimeException("Stub!");
    }

    public final boolean sendMessage(Message msg) {
        throw new RuntimeException("Stub!");
    }

    public final boolean sendEmptyMessage(int what) {
        throw new RuntimeException("Stub!");
    }

    public void handleMessage(Message msg) {
        throw new RuntimeException("Stub!");
    }

    public static interface Callback {
        boolean handleMessage(Message msg);
    }
}
