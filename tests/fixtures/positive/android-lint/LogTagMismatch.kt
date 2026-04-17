package com.example

import android.util.Log

class MyActivity {
    companion object {
        const val TAG = "WrongName"
    }

    fun doWork() {
        Log.d(TAG, "message")
    }
}
