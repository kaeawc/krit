package com.example

import android.app.Activity
import android.os.Binder
import android.os.Bundle
import android.os.Process

class CallerActivity : Activity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val uid = Process.myUid()
    }

    fun elsewhere() {
        val uid = Binder.getCallingUid()
    }
}
