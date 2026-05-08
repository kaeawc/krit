package com.example

import android.content.Context
import android.widget.GridLayout

class MyActivity {
    fun setupGrid(context: Context) {
        val grid = GridLayout(context)
        grid.columnCount = 3
        grid.addView(null)
    }
}
