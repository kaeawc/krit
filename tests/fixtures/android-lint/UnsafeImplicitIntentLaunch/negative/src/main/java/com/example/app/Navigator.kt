package com.example.app

import android.content.Context
import android.content.Intent

class Navigator(private val context: Context) {
    fun openSettings() {
        context.startActivity(Intent("com.example.app.OPEN_SETTINGS").setPackage(context.packageName))
    }

    fun openSettingsExplicitly() {
        context.startActivity(Intent(context, SettingsActivity::class.java))
    }

    fun openPublicScreen() {
        context.startActivity(Intent("com.example.app.SHARE"))
    }
}

class SettingsActivity : android.app.Activity()
