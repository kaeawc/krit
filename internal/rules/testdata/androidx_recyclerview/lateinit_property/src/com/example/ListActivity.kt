package com.example

import android.app.Activity
import android.os.Bundle
import androidx.recyclerview.widget.RecyclerView

class ListActivity : Activity() {
    private lateinit var list: RecyclerView
    private lateinit var grid: RecyclerView

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.activity_main)
        list = findViewById(R.id.list)
        grid = findViewById(R.id.grid)
        configure()
    }

    private fun configure() {
        list.setHasFixedSize(true)
        grid.setHasFixedSize(true)
    }
}
