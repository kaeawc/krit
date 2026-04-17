package com.example

import android.view.LayoutInflater

class MyAdapter {
    fun createView(inflater: LayoutInflater) {
        val view = inflater.inflate(R.layout.item_row, null)
    }
}
