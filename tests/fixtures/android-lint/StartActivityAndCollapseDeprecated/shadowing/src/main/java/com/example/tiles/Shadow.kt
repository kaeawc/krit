package com.example.tiles

import android.app.PendingIntent
import android.content.Intent
import android.service.quicksettings.TileService

class Collapser {
    fun startActivityAndCollapse(intent: Intent) {}
}

class ShadowTile : TileService() {
    private val collapser = Collapser()

    fun helper(intent: Intent) {}

    override fun onClick() {
        val intent = PendingIntent.getActivity(this, 0, Intent(), PendingIntent.FLAG_IMMUTABLE)
        startActivityAndCollapse(intent)
        with(collapser) {
            startActivityAndCollapse(Intent())
        }
        collapser.apply { startActivityAndCollapse(Intent()) }
    }

    override fun onStartListening() {
        fun startActivityAndCollapse(i: Intent) {}
        startActivityAndCollapse(Intent())
    }
}
