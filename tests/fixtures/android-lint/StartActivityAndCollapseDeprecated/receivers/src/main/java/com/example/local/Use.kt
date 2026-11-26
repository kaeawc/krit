package com.example.local

import android.content.Intent

fun h(s: TileService, i: Intent) {
    s.startActivityAndCollapse(i)
}
