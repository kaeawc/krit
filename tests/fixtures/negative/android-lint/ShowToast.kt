package com.example

import android.content.Context
import android.widget.Toast

class MyActivity {
    fun notify(context: Context) {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).show()
    }
}
