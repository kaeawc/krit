package com.example

import android.content.Context
import android.view.View

class LabelView(context: Context) : View(context) {
    override fun getContentDescription(): CharSequence = "label"
}
