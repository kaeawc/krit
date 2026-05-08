package com.example

import android.view.LayoutInflater

class MyAdapter {
    fun createView(inflater: LayoutInflater) {
        val view = inflater.inflate(R.layout.with_root_layout_params, null)
    }
}
