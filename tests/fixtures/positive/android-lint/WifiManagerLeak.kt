package com.example

import android.app.Activity
import android.content.Context

class WifiActivity : Activity() {
    fun wifi() {
        getSystemService(Context.WIFI_SERVICE)
    }
}
