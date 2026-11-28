package com.example

import android.content.Context
import android.content.pm.ShortcutInfo
import android.content.pm.ShortcutManager
import androidx.core.content.getSystemService

class Helper(private val service: Any?, private val label: String) {
    fun setDynamicShortcuts(list: List<ShortcutInfo>) {}
}

class Receivers(private val context: Context) {
    private val lazyManager by lazy { context.getSystemService(ShortcutManager::class.java) }

    // Mentions both words, but it is a Helper.
    fun textOnly(list: List<ShortcutInfo>) {
        val m = Helper(context.getSystemService("x"), "ShortcutManager")
        m.setDynamicShortcuts(list)
    }

    fun chained(list: List<ShortcutInfo>) {
        context.getSystemService(ShortcutManager::class.java)!!.addDynamicShortcuts(list)
    }

    fun lazy(list: List<ShortcutInfo>) {
        lazyManager.setDynamicShortcuts(list)
    }

    fun reified(list: List<ShortcutInfo>) {
        val m = context.getSystemService<ShortcutManager>()
        m?.setDynamicShortcuts(list)
    }

    fun cast(list: List<ShortcutInfo>) {
        val m = context.getSystemService(Context.SHORTCUT_SERVICE) as ShortcutManager
        m.setDynamicShortcuts(list)
    }
}
