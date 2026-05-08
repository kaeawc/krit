package com.example

import android.content.Context
import android.net.ConnectivityManager

class NetworkMonitor(private val context: Context) {
    fun checkWifi() {
        // WIFI_SERVICE should be cast to WifiManager, not ConnectivityManager
        val cm = context.getSystemService(Context.WIFI_SERVICE) as ConnectivityManager
        val network = cm.activeNetwork
    }
}
