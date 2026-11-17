package com.example

import android.app.Activity
import android.content.Context

class WifiActivity : Activity() {
    fun wifi() {
        applicationContext.getSystemService(Context.WIFI_SERVICE)
    }
}
