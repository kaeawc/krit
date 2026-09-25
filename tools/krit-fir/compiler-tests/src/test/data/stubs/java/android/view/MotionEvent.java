// Compiler-test source stub; never packaged in the production artifact.
package android.view;

public final class MotionEvent {
    public static final int ACTION_DOWN = 0;

    public static final int ACTION_UP = 1;

    public static final int ACTION_MOVE = 2;

    public static final int ACTION_CANCEL = 3;

    MotionEvent() {
    }

    public static MotionEvent obtain(long downTime, long eventTime, int action, float x, float y, int metaState) {
        throw new RuntimeException("Stub!");
    }

    public final int getAction() {
        throw new RuntimeException("Stub!");
    }

    public final float getX() {
        throw new RuntimeException("Stub!");
    }

    public final float getY() {
        throw new RuntimeException("Stub!");
    }

    public final void recycle() {
        throw new RuntimeException("Stub!");
    }
}
