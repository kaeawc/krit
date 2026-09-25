// Compiler-test source stubs; never packaged in the production artifact.
package android.location

object LocationManager {
    const val GPS_PROVIDER: String = "gps"
    const val NETWORK_PROVIDER: String = "network"
    const val FUSED_PROVIDER: String = "fused"

    fun getLastKnownLocation(provider: String): Location? = TODO()

    fun requestLocationUpdates(provider: String, minTimeMs: Long, minDistanceM: Float, listener: LocationListener) {
        TODO()
    }

    fun removeUpdates(listener: LocationListener) {
        TODO()
    }

    fun isProviderEnabled(provider: String): Boolean = TODO()
}

// Java interface with one abstract method and default methods: a SAM type.
fun interface LocationListener {
    fun onLocationChanged(location: Location)

    fun onProviderEnabled(provider: String) {}

    fun onProviderDisabled(provider: String) {}
}

open class Location(provider: String?) {
    open var latitude: Double
        get() = TODO()
        set(value) = TODO()

    open var longitude: Double
        get() = TODO()
        set(value) = TODO()

    open var accuracy: Float
        get() = TODO()
        set(value) = TODO()

    open val time: Long
        get() = TODO()
}
