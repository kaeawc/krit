// Compiler-test source stubs; never packaged in the production artifact.
package android.bluetooth

// Java final class with a static getDefaultAdapter() and instance methods;
// modeled as an object so both keep their Java callable ids.
object BluetoothAdapter {
    const val ACTION_REQUEST_ENABLE: String = "android.bluetooth.adapter.action.REQUEST_ENABLE"

    val isEnabled: Boolean
        get() = TODO()

    @Deprecated("Deprecated in Java")
    fun getDefaultAdapter(): BluetoothAdapter? = TODO()

    @Deprecated("Deprecated in Java")
    fun enable(): Boolean = TODO()

    fun getRemoteDevice(address: String): BluetoothDevice = TODO()
}

object BluetoothManager {
    val adapter: BluetoothAdapter?
        get() = TODO()
}

class BluetoothDevice {
    val address: String
        get() = TODO()

    val name: String?
        get() = TODO()
}
