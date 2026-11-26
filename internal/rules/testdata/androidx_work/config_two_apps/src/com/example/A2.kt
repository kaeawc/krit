package com.example

import android.app.Application
import androidx.work.Configuration

open class App2 : Application()

class Config : Configuration.Provider {
    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()
}
