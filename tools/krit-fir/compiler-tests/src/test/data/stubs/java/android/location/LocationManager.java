// Compiler-test source stub; never packaged in the production artifact.
package android.location;

public class LocationManager {
    public static final String GPS_PROVIDER = "gps";

    public static final String NETWORK_PROVIDER = "network";

    public static final String FUSED_PROVIDER = "fused";

    LocationManager() {
    }

    public Location getLastKnownLocation(String provider) {
        throw new RuntimeException("Stub!");
    }

    public void requestLocationUpdates(String provider, long minTimeMs, float minDistanceM, LocationListener listener) {
        throw new RuntimeException("Stub!");
    }

    public void removeUpdates(LocationListener listener) {
        throw new RuntimeException("Stub!");
    }

    public boolean isProviderEnabled(String provider) {
        throw new RuntimeException("Stub!");
    }
}
