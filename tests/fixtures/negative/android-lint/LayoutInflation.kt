package com.example

import android.view.LayoutInflater
import android.view.ViewGroup

class MyAdapter {
    fun createView(inflater: LayoutInflater, parent: ViewGroup) {
        val view = inflater.inflate(R.layout.item_row, parent, false)
    }
}
