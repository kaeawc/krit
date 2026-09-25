// Compiler-test source stub; never packaged in the production artifact.
package android.content.res;

import android.graphics.drawable.Drawable;

public class Resources {
    @Deprecated
    public Resources() {
    }

    public static Resources getSystem() {
        throw new RuntimeException("Stub!");
    }

    public Configuration getConfiguration() {
        throw new RuntimeException("Stub!");
    }

    public String getString(int id) throws NotFoundException {
        throw new RuntimeException("Stub!");
    }

    public String getString(int id, Object... formatArgs) throws NotFoundException {
        throw new RuntimeException("Stub!");
    }

    public CharSequence getText(int id) throws NotFoundException {
        throw new RuntimeException("Stub!");
    }

    public String getQuantityString(int id, int quantity, Object... formatArgs) throws NotFoundException {
        throw new RuntimeException("Stub!");
    }

    public String[] getStringArray(int id) throws NotFoundException {
        throw new RuntimeException("Stub!");
    }

    public int getColor(int id, Theme theme) throws NotFoundException {
        throw new RuntimeException("Stub!");
    }

    public int getDimensionPixelSize(int id) throws NotFoundException {
        throw new RuntimeException("Stub!");
    }

    public Drawable getDrawable(int id, Theme theme) throws NotFoundException {
        throw new RuntimeException("Stub!");
    }

    public int getIdentifier(String name, String defType, String defPackage) {
        throw new RuntimeException("Stub!");
    }

    public final class Theme {
        Theme() {
        }

        public TypedArray obtainStyledAttributes(int[] attrs) {
            throw new RuntimeException("Stub!");
        }
    }

    public static class NotFoundException extends RuntimeException {
        public NotFoundException() {
        }

        public NotFoundException(String name) {
        }
    }
}
