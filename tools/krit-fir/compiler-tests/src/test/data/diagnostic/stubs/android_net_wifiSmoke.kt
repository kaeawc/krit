// Smoke: WifiManager obtained from the Context.WIFI_SERVICE constant.
package stubs

import android.content.Context
import android.net.wifi.WifiInfo
import android.net.wifi.WifiManager

fun currentWifi(context: Context): WifiInfo? {
    val manager = context.getSystemService(Context.WIFI_SERVICE) as WifiManager
    return manager.connectionInfo
}
