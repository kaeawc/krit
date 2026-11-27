package com.example

import android.app.Activity
import android.view.View
import java.util.Objects

class KCasts : Activity() {
    fun bind() {
        Objects.requireNonNull(findViewById<View>(R.id.title)).visibility = View.GONE
    }
}
