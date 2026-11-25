package com.example.synth

import androidx.work.Configuration

class WorkConfig : Configuration.Provider {
    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()
}
