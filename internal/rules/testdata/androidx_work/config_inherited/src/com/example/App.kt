package com.example

import androidx.work.Configuration

class App : BaseApp() {
    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()
}
