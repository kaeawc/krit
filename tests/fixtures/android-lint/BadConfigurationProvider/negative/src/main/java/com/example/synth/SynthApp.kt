package com.example.synth

import android.app.Application
import androidx.work.Configuration

class SynthApp : Application(), Configuration.Provider {
    override val workManagerConfiguration: Configuration
        get() = Configuration.Builder().build()
}
