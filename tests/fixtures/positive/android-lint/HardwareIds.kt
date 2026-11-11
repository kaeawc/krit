package com.example

import android.content.Context
import android.provider.Settings

class DeviceInfo(private val context: Context) {
    fun id(): String? = Settings.Secure.getString(context.contentResolver, Settings.Secure.ANDROID_ID)
}
