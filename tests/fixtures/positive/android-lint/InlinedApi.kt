package com.example

import android.view.View

class MyActivity {
    fun hideSystemUi(view: View) {
        view.systemUiVisibility = View.SYSTEM_UI_FLAG_IMMERSIVE_STICKY or View.SYSTEM_UI_FLAG_FULLSCREEN
    }
}
