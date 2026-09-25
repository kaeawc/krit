// Compiler-test source stub; never packaged in the production artifact.
package android.graphics;

public class Paint {
    public static final int ANTI_ALIAS_FLAG = 1;

    public Paint() {
    }

    public Paint(int flags) {
    }

    public int getColor() {
        throw new RuntimeException("Stub!");
    }

    public void setColor(int color) {
        throw new RuntimeException("Stub!");
    }

    public final boolean isAntiAlias() {
        throw new RuntimeException("Stub!");
    }

    public void setAntiAlias(boolean aa) {
        throw new RuntimeException("Stub!");
    }

    public float getTextSize() {
        throw new RuntimeException("Stub!");
    }

    public void setTextSize(float textSize) {
        throw new RuntimeException("Stub!");
    }

    public float getStrokeWidth() {
        throw new RuntimeException("Stub!");
    }

    public void setStrokeWidth(float width) {
        throw new RuntimeException("Stub!");
    }

    public Style getStyle() {
        throw new RuntimeException("Stub!");
    }

    public void setStyle(Style style) {
        throw new RuntimeException("Stub!");
    }

    public static enum Style {
        FILL,
        STROKE,
        FILL_AND_STROKE,
    }
}
