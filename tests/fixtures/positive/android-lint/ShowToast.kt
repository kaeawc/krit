package com.example

import android.content.Context
import android.widget.Toast

class MyActivity {
    fun notify(context: Context) {
        Toast.makeText(context, "Hello", Toast.LENGTH_SHORT)
        // A literal duration is reported even when the toast is shown.
        Toast.makeText(context, "Hello", 2000).show()
    }
}
