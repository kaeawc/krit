package com.example

import android.app.Activity

class FixedSizeHolder {
    fun setHasFixedSize(value: Boolean) = Unit
}

class LookalikeActivity : Activity() {
    fun bind(holder: FixedSizeHolder) {
        findViewById<android.view.View>(R.id.list)
        holder.setHasFixedSize(true)
    }
}
