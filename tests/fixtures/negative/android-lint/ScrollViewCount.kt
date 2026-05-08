package com.example

import android.content.Context
import android.view.View
import android.widget.LinearLayout
import android.widget.ScrollView

fun build(context: Context) {
    // Single direct child — idiomatic ScrollView usage.
    ScrollView(context).apply {
        addView(LinearLayout(context).apply {
            addView(View(context))
            addView(View(context))
        })
    }

    // Multiple addViews on a non-scrollable container — fine.
    LinearLayout(context).apply {
        addView(View(context))
        addView(View(context))
    }
}
