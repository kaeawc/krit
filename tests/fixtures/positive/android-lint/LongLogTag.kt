package com.example

import android.util.Log

class MyClass {
    fun doWork() {
        Log.d("VeryLongTagNameThatExceedsLimit", "message")
    }
}
