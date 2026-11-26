package com.example

import android.content.Context
import android.content.pm.ShortcutInfo
import android.content.pm.ShortcutManager
import androidx.core.content.pm.ShortcutInfoCompat
import androidx.core.content.pm.ShortcutManagerCompat

fun publish(context: Context, shortcuts: List<ShortcutInfo>, compat: List<ShortcutInfoCompat>) {
    val manager = context.getSystemService(ShortcutManager::class.java)
    manager.setDynamicShortcuts(shortcuts)
    manager.addDynamicShortcuts(shortcuts)
    ShortcutManagerCompat.setDynamicShortcuts(context, compat)
}
