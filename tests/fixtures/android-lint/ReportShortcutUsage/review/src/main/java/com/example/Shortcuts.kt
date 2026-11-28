package com.example

import android.content.Context
import android.content.pm.ShortcutInfo
import android.content.pm.ShortcutManager

class FakeShortcuts {
    fun setDynamicShortcuts(list: List<ShortcutInfo>) {}
}

class Shadows(context: Context) {
    private val manager: ShortcutManager = context.getSystemService(ShortcutManager::class.java)

    // A lambda parameter hides the local.
    fun lambdaParam(context: Context, fakes: List<FakeShortcuts>, shortcuts: List<ShortcutInfo>) {
        val manager = context.getSystemService(ShortcutManager::class.java)
        fakes.forEach { manager -> manager.setDynamicShortcuts(shortcuts) }
        println(manager)
    }

    // A loop variable hides the property.
    fun loopVariable(fakes: List<FakeShortcuts>, shortcuts: List<ShortcutInfo>) {
        for (manager in fakes) {
            manager.setDynamicShortcuts(shortcuts)
        }
        println(manager)
    }
}
