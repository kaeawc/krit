// Compiler-test source stub; never packaged in the production artifact.
package android.widget;

import android.content.Context;

public class Toast {
    public static final int LENGTH_SHORT = 0;

    public static final int LENGTH_LONG = 1;

    public Toast(Context context) {
    }

    public static Toast makeText(Context context, CharSequence text, int duration) {
        throw new RuntimeException("Stub!");
    }

    public static Toast makeText(Context context, int resId, int duration) {
        throw new RuntimeException("Stub!");
    }

    public void show() {
        throw new RuntimeException("Stub!");
    }

    public void cancel() {
        throw new RuntimeException("Stub!");
    }

    public int getDuration() {
        throw new RuntimeException("Stub!");
    }

    public void setDuration(int duration) {
        throw new RuntimeException("Stub!");
    }

    public void setText(CharSequence s) {
        throw new RuntimeException("Stub!");
    }
}
