package com.example

import android.app.Activity
import androidx.recyclerview.widget.RecyclerView

class GridActivity : Activity() {
    fun bind() {
        val grid = findViewById<RecyclerView>(R.id.grid)
        grid.setHasFixedSize(true)
        val cast = findViewById(R.id.grid) as RecyclerView
        cast.setHasFixedSize(true)
    }
}
