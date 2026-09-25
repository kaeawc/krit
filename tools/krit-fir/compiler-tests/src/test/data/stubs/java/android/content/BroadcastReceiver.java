// Compiler-test source stub; never packaged in the production artifact.
package android.content;

public abstract class BroadcastReceiver {
    public BroadcastReceiver() {
    }

    public abstract void onReceive(Context context, Intent intent);

    public final PendingResult goAsync() {
        throw new RuntimeException("Stub!");
    }

    public final boolean isOrderedBroadcast() {
        throw new RuntimeException("Stub!");
    }

    public static class PendingResult {
        PendingResult() {
        }

        public final void finish() {
            throw new RuntimeException("Stub!");
        }
    }
}
