// Compiler-test source stub; never packaged in the production artifact.
package android.view;

import android.content.Context;

public abstract class LayoutInflater {
    protected LayoutInflater(Context context) {
    }

    public static LayoutInflater from(Context context) {
        throw new RuntimeException("Stub!");
    }

    public View inflate(int resource, ViewGroup root) {
        throw new RuntimeException("Stub!");
    }

    public View inflate(int resource, ViewGroup root, boolean attachToRoot) {
        throw new RuntimeException("Stub!");
    }

    public abstract LayoutInflater cloneInContext(Context newContext);
}
