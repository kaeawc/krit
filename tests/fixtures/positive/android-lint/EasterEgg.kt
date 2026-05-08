package com.example

class SettingsScreen {
    // TODO: remove this easter egg before release
    fun onTapLogo(count: Int) {
        if (count >= 7) {
            showHiddenMenu()
        }
    }
}
