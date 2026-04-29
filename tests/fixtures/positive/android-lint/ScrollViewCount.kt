package com.example

import android.content.Context
import android.view.View
import android.widget.HorizontalScrollView
import android.widget.ScrollView

fun build(context: Context) {
    ScrollView(context).apply {
        addView(View(context))
        addView(View(context))
    }

    HorizontalScrollView(context).apply {
        addView(View(context))
        addView(View(context))
        addView(View(context))
    }
}
