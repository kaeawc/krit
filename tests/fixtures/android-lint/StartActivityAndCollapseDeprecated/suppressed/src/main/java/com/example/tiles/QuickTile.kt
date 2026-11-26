package com.example.tiles

import android.content.Intent
import android.service.quicksettings.TileService

class QuickTile : TileService() {
    @Suppress("StartActivityAndCollapseDeprecated")
    override fun onClick() {
        val intent = Intent(this, SettingsActivity::class.java)
        startActivityAndCollapse(intent)
        startActivityAndCollapse(Intent(this, SettingsActivity::class.java))
        this.startActivityAndCollapse(intent.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK))
    }
}

class SettingsActivity : android.app.Activity()

fun launchFrom(service: TileService, intent: Intent) {
    service.startActivityAndCollapse(intent)
}
