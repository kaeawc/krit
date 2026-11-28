package com.example

import android.content.Context
import android.content.pm.ShortcutInfo
import android.content.pm.ShortcutManager

class Fake {
    fun setDynamicShortcuts(items: List<ShortcutInfo>) {}
}

class Shortcuts(private val context: Context) {
    val shortcuts by lazy { context.getSystemService(ShortcutManager::class.java) }

    // The parameter hides the lazy property.
    fun fake(shortcuts: Fake, items: List<ShortcutInfo>) {
        shortcuts.setDynamicShortcuts(items)
    }

    fun real(items: List<ShortcutInfo>) {
        shortcuts.setDynamicShortcuts(items)
    }
}
