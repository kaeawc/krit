// Compiler-test source stub; never packaged in the production artifact.
package android.app;

import android.content.ContextWrapper;

public class Application extends ContextWrapper {
    public Application() {
        super(null);
    }

    public void onCreate() {
        throw new RuntimeException("Stub!");
    }

    public void onTerminate() {
        throw new RuntimeException("Stub!");
    }

    public void onLowMemory() {
        throw new RuntimeException("Stub!");
    }
}
