package test

import android.location.LocationManager
import androidx.core.content.ContextCompat

class LocationTracker {
    fun startTracking(manager: LocationManager) {
        if (ContextCompat.checkSelfPermission(context, ACCESS_FINE_LOCATION) == GRANTED) {
            manager.requestLocationUpdates("gps", 1000L, 0f, listener)
        }
    }
}
