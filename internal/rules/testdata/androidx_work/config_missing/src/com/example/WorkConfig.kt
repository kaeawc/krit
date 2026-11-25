package com.example

import androidx.work.Configuration

class WorkConfig : Configuration.Provider {
    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()
}
