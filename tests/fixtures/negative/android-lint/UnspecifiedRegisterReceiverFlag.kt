package com.example

import android.app.Activity
import android.content.BroadcastReceiver
import android.content.Context
import android.content.IntentFilter

class Screen : Activity() {
    private val receiver: BroadcastReceiver? = null

    fun listen() {
        registerReceiver(receiver, IntentFilter("com.example.UPDATED"), Context.RECEIVER_NOT_EXPORTED)
    }
}
