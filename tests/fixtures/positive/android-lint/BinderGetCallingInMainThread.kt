package com.example

import android.app.Activity
import android.os.Binder
import android.os.Bundle

class CallerActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val uid = Binder.getCallingUid()
    }
}
