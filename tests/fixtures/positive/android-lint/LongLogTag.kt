package com.example

import android.util.Log

class MyClass {
    companion object {
        private const val TAG = "NetworkRepositoryImpl_v2"
    }

    fun doWork() {
        Log.d("VeryLongTagNameThatExceedsLimit", "message")
        Log.i(TAG, "message")
    }
}
