package com.example.synth

import android.app.Activity
import android.os.Bundle
import androidx.recyclerview.widget.RecyclerView

class Main : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        setContentView(R.layout.main)
        findViewById<RecyclerView>(R.id.list).setHasFixedSize(true)
    }
}
