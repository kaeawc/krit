package com.example

import android.content.Context
import android.view.View

class LabelView(context: Context) : View(context) {
    init {
        contentDescription = "label"
    }
}

class Describer {
    fun getContentDescription(): CharSequence = "label"
}
