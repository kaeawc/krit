package com.example

import android.app.Activity

class MainActivity : Activity() {
    override fun onBackPressed() {
        finish()
    }
}
