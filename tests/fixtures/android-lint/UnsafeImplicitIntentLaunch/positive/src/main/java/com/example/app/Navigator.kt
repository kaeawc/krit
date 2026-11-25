package com.example.app

import android.content.Context
import android.content.Intent

class Navigator(private val context: Context) {
    fun openSettings() {
        context.startActivity(Intent("com.example.app.OPEN_SETTINGS"))
    }
}
