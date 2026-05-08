package com.example

import android.os.Handler
import android.os.Looper

class MyActivity {
    // Not an inner class, so no implicit reference to outer
    class SafeHandler(looper: Looper) : Handler(looper) {
        override fun handleMessage(msg: android.os.Message) {
            // handle safely
        }
    }

    inner class LooperHandler(looper: Looper) : Handler(looper) {
        override fun handleMessage(msg: android.os.Message) {
            // handle safely
        }
    }
}
