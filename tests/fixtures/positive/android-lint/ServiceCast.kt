package com.example

import android.content.Context

class MyActivity {
    fun setup(context: Context) {
        val manager = context.getSystemService(Context.ALARM_SERVICE) as PowerManager
    }
}
