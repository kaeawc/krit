package com.example

import android.content.Context
import android.net.ConnectivityManager
import android.net.wifi.WifiManager

class NetworkMonitor(private val context: Context) {
    fun checkWifi() {
        val wm = context.getSystemService(Context.WIFI_SERVICE) as WifiManager
        val info = wm.connectionInfo
    }

    fun checkConnectivity() {
        val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val network = cm.activeNetwork
    }
}
