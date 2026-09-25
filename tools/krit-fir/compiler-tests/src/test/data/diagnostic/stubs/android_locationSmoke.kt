// Smoke: location updates via a SAM-converted LocationListener.
package stubs

import android.content.Context
import android.location.Location
import android.location.LocationListener
import android.location.LocationManager

@Suppress("MissingPermission")
fun lastLocation(context: Context): Location? {
    val manager: LocationManager = context.getSystemService(LocationManager::class.java) ?: return null
    if (!manager.isProviderEnabled(LocationManager.GPS_PROVIDER)) return null
    val listener = LocationListener { location -> println(location.latitude + location.longitude) }
    manager.requestLocationUpdates(LocationManager.GPS_PROVIDER, 1000L, 1f, listener)
    manager.removeUpdates(listener)
    return manager.getLastKnownLocation(LocationManager.NETWORK_PROVIDER)
}
