// Compiler-test source stub; never packaged in the production artifact.
package android.os;

public final class Message {
    public int what;

    public int arg1;

    public int arg2;

    public Object obj;

    public Message() {
    }

    public static Message obtain() {
        throw new RuntimeException("Stub!");
    }

    public static Message obtain(Handler h, int what) {
        throw new RuntimeException("Stub!");
    }

    public void sendToTarget() {
        throw new RuntimeException("Stub!");
    }
}
