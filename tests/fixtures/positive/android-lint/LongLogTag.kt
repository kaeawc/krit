package com.example

import android.util.Log

class MyClass {
    fun doWork() {
        Log.d("VeryLongTagNameThatExceedsLimit", "message")
        if (Log.isLoggable("VeryLongTagNameThatExceedsLimit", Log.DEBUG)) {
            Log.println(Log.DEBUG, "VeryLongTagNameThatExceedsLimit", "message")
        }
    }
}
