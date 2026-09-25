// Compiler-test source stub; never packaged in the production artifact.
package android.content.pm;

import android.content.Intent;
import android.util.AndroidException;

public abstract class PackageManager {
    public static final int PERMISSION_GRANTED = 0;

    public static final int PERMISSION_DENIED = -1;

    public static final int GET_META_DATA = 128;

    public static final String FEATURE_CAMERA = "android.hardware.camera";

    public PackageManager() {
    }

    public abstract PackageInfo getPackageInfo(String packageName, int flags) throws NameNotFoundException;

    public abstract int checkPermission(String permName, String packageName);

    public abstract boolean hasSystemFeature(String featureName);

    public abstract Intent getLaunchIntentForPackage(String packageName);

    public static class NameNotFoundException extends AndroidException {
        public NameNotFoundException() {
        }

        public NameNotFoundException(String name) {
        }
    }
}
