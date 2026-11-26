package com.example.tiles

import android.app.PendingIntent
import android.content.Intent
import android.service.quicksettings.TileService

class QuickTile : TileService() {
    override fun onClick() {
        val intent = Intent(this, SettingsActivity::class.java)
        val pending = PendingIntent.getActivity(this, 0, intent, PendingIntent.FLAG_IMMUTABLE)
        startActivityAndCollapse(pending)
        // startActivityAndCollapse(intent)
    }
}

class SettingsActivity : android.app.Activity()

class Collapser {
    fun startActivityAndCollapse(intent: Intent) {}

    fun run(intent: Intent) {
        startActivityAndCollapse(intent)
    }
}

fun notATile(collapser: Collapser, intent: Intent) {
    collapser.startActivityAndCollapse(intent)
}
