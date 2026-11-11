package com.example

import android.content.Context
import android.provider.Settings

class DeviceInfo(private val context: Context) {
    fun name(): String? = Settings.Global.getString(context.contentResolver, Settings.Global.DEVICE_NAME)
}
