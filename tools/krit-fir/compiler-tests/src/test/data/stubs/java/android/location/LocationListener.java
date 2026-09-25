// Compiler-test source stub; never packaged in the production artifact.
package android.location;

public interface LocationListener {
    void onLocationChanged(Location location);

    default void onProviderEnabled(String provider) {
        throw new RuntimeException("Stub!");
    }

    default void onProviderDisabled(String provider) {
        throw new RuntimeException("Stub!");
    }
}
