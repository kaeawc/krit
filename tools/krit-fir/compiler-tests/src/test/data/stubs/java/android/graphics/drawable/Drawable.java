// Compiler-test source stub; never packaged in the production artifact.
package android.graphics.drawable;

import android.graphics.Canvas;
import android.graphics.ColorFilter;

public abstract class Drawable {
    public Drawable() {
    }

    public int getIntrinsicWidth() {
        throw new RuntimeException("Stub!");
    }

    public int getIntrinsicHeight() {
        throw new RuntimeException("Stub!");
    }

    public abstract void draw(Canvas canvas);

    public abstract void setAlpha(int alpha);

    public abstract void setColorFilter(ColorFilter colorFilter);

    @Deprecated
    public abstract int getOpacity();

    public void setBounds(int left, int top, int right, int bottom) {
        throw new RuntimeException("Stub!");
    }

    public void setTint(int tintColor) {
        throw new RuntimeException("Stub!");
    }
}
