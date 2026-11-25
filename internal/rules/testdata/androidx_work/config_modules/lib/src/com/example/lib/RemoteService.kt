package com.example.lib

import android.app.Service
import android.content.Intent
import androidx.work.Configuration

class RemoteService : Service(), Configuration.Provider {
    override fun onBind(intent: Intent?) = null

    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()
}
