package com.example

import android.os.Handler

class MyActivity {
    inner class MyHandler : Handler() {
        override fun handleMessage(msg: android.os.Message) {
            // handle
        }
    }
}
