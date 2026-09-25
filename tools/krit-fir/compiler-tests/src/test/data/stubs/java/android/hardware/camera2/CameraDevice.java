// Compiler-test source stub; never packaged in the production artifact.
package android.hardware.camera2;

public abstract class CameraDevice implements AutoCloseable {
    CameraDevice() {
    }

    public abstract String getId();

    public abstract void close();

    public abstract static class StateCallback {
        public StateCallback() {
        }

        public abstract void onOpened(CameraDevice camera);

        public abstract void onDisconnected(CameraDevice camera);

        public abstract void onError(CameraDevice camera, int error);

        public void onClosed(CameraDevice camera) {
            throw new RuntimeException("Stub!");
        }
    }
}
