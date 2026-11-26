package com.example

import android.app.Activity
import android.os.Bundle
import androidx.recyclerview.widget.RecyclerView

class MainActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        val list = findViewById<RecyclerView>(R.id.list)
        list.setHasFixedSize(true)
        list.setHasFixedSize(false)
        findViewById<RecyclerView>(R.id.wide).setHasFixedSize(true)
        val custom = findViewById<RecyclerView>(R.id.custom)
        custom.setHasFixedSize(true)
    }

    fun other(grid: RecyclerView) {
        grid.setHasFixedSize(true)
    }
}
