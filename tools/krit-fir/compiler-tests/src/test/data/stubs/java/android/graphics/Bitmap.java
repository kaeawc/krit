// Compiler-test source stub; never packaged in the production artifact.
package android.graphics;

public final class Bitmap {
    Bitmap() {
    }

    public int getWidth() {
        throw new RuntimeException("Stub!");
    }

    public int getHeight() {
        throw new RuntimeException("Stub!");
    }

    public void recycle() {
        throw new RuntimeException("Stub!");
    }

    public final Config getConfig() {
        throw new RuntimeException("Stub!");
    }

    public enum Config {
        ALPHA_8,
        RGB_565,
        @Deprecated
        ARGB_4444,
        ARGB_8888,
        RGBA_F16,
        HARDWARE,
        RGBA_1010102
    }
}
