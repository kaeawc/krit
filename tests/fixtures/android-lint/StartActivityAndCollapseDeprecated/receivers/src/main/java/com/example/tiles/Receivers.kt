package com.example.tiles

import android.app.PendingIntent
import android.content.Intent
import android.service.quicksettings.TileService

class Collapser2 {
    fun startActivityAndCollapse(intent: Intent) {}
}

class ReceiverTile : TileService() {
    fun Collapser2.go() {
        startActivityAndCollapse(Intent())
    }

    fun plainRun() {
        run { startActivityAndCollapse(Intent()) }
    }

    fun intentApply() {
        Intent().apply { startActivityAndCollapse(this) }
    }

    fun labeled() {
        listOf(1).forEach { this@ReceiverTile.startActivityAndCollapse(Intent()) }
    }

    fun superCall() {
        super.startActivityAndCollapse(Intent())
    }

    fun qualified() {
        startActivityAndCollapse(android.content.Intent())
    }

    fun localShadow() {
        fun startActivityAndCollapse(i: Intent) {}
        this.startActivityAndCollapse(Intent())
    }

    fun closedBlock(action: PendingIntent, enabled: Boolean) {
        if (enabled) {
            val action = Intent()
            println(action)
        }
        startActivityAndCollapse(action)
    }
}

fun TileService.ext(i: Intent) {
    startActivityAndCollapse(i)
}
