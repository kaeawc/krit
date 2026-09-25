// Compiler-test source stub; never packaged in the production artifact.
package android.bluetooth;

public final class BluetoothAdapter {
    public static final String ACTION_REQUEST_ENABLE = "android.bluetooth.adapter.action.REQUEST_ENABLE";

    BluetoothAdapter() {
    }

    @Deprecated
    public static synchronized BluetoothAdapter getDefaultAdapter() {
        throw new RuntimeException("Stub!");
    }

    public boolean isEnabled() {
        throw new RuntimeException("Stub!");
    }

    @Deprecated
    public boolean enable() {
        throw new RuntimeException("Stub!");
    }

    public BluetoothDevice getRemoteDevice(String address) {
        throw new RuntimeException("Stub!");
    }
}
