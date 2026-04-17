package test

import android.location.LocationManager

class LocationTracker {
    fun startTracking(manager: LocationManager) {
        manager.requestLocationUpdates("gps", 1000L, 0f, listener)
    }
}
