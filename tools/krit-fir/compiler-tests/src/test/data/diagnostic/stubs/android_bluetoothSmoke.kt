// Smoke: reach the Bluetooth adapter through the system service and the legacy static.
package stubs

import android.bluetooth.BluetoothAdapter
import android.bluetooth.BluetoothManager
import android.content.Context

fun bluetoothEnabled(context: Context): Boolean {
    val manager: BluetoothManager? = context.getSystemService(BluetoothManager::class.java)
    @Suppress("DEPRECATION")
    val legacy: BluetoothAdapter? = BluetoothAdapter.getDefaultAdapter()
    val adapter: BluetoothAdapter? = manager?.adapter ?: legacy
    return adapter?.isEnabled == true
}

fun enableIntentAction(): String = BluetoothAdapter.ACTION_REQUEST_ENABLE
