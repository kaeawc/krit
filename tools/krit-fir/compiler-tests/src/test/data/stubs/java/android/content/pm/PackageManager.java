// Compiler-test source stub; never packaged in the production artifact.
package android.content.pm;

import android.content.Intent;
import android.util.AndroidException;

public abstract class PackageManager {
    public static final int PERMISSION_GRANTED = 0;

    public static final int PERMISSION_DENIED = -1;

    public static final int GET_META_DATA = 128;

    @Deprecated
    public static final int GET_SIGNATURES = 64;

    public static final int GET_SIGNING_CERTIFICATES = 134217728;

    public static final String FEATURE_CAMERA = "android.hardware.camera";

    public PackageManager() {
    }

    public abstract PackageInfo getPackageInfo(String packageName, int flags) throws NameNotFoundException;

    public PackageInfo getPackageInfo(String packageName, PackageInfoFlags flags) throws NameNotFoundException {
        throw new RuntimeException("Stub!");
    }

    public abstract int checkPermission(String permName, String packageName);

    public abstract boolean hasSystemFeature(String featureName);

    public abstract Intent getLaunchIntentForPackage(String packageName);

    // The SDK class extends the hidden PackageManager.Flags; only the public
    // surface is modeled.
    public static final class PackageInfoFlags {
        PackageInfoFlags() {
        }

        public static PackageInfoFlags of(long value) {
            throw new RuntimeException("Stub!");
        }

        public long getValue() {
            throw new RuntimeException("Stub!");
        }
    }

    public static class NameNotFoundException extends AndroidException {
        public NameNotFoundException() {
        }

        public NameNotFoundException(String name) {
        }
    }
}
