// Compiler-test source stub; never packaged in the production artifact.
package android.hardware.camera2;

import android.util.AndroidException;

public class CameraAccessException extends AndroidException {
    public CameraAccessException(int problem) {
    }

    public final int getReason() {
        throw new RuntimeException("Stub!");
    }
}
