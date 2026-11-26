package com.example

import android.app.Application
import androidx.work.Configuration

class App : Application(), Configuration.Provider {
    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()
}
