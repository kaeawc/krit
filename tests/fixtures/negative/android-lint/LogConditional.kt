package com.example

import android.util.Log

class MyService {
    fun doWork() {
        if (Log.isLoggable("MyService", Log.DEBUG))
            Log.d("MyService", "starting work")
    }
}
