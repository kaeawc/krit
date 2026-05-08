package com.example.app.fragments

import android.os.Bundle
import androidx.preference.PreferenceFragmentCompat
import com.example.app.R

class SettingsFragment : PreferenceFragmentCompat() {

    override fun onCreatePreferences(savedInstanceState: Bundle?, rootKey: String?) {
        setPreferencesFromResource(R.xml.preferences, rootKey)
    }

    // MagicNumber
    fun getMaxCacheSize(): Int {
        return 50 * 1024 * 1024
    }

    // MagicNumber
    fun getRetryInterval(): Long {
        return 30000L
    }
}
