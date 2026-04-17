package com.example.feature.settings

// Only referenced by SettingsFragment - low fan-in, not a hotspot
class ThemePreference(private val prefs: SharedPreferences) {
    fun isDarkMode(): Boolean = prefs.getBoolean("dark_mode", false)
    fun setDarkMode(enabled: Boolean) = prefs.edit().putBoolean("dark_mode", enabled).apply()
}
