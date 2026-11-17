// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
// The cast target is compared by its qualified name, as Android Lint compares it.
package test.servicecast.qualified

import android.content.Context

// A project class that shares the platform manager's simple name.
class AlarmManager

class Qualified(private val context: Context) {
    // Go compares the last identifier of the written name and misses this; the cast fails at run time.
    fun projectClassWithManagerName() {
        val am = <!ServiceCast!>context.getSystemService(Context.ALARM_SERVICE) as AlarmManager<!>
        println(am)
    }

    fun platformManager() {
        val am = context.getSystemService(Context.ALARM_SERVICE) as android.app.AlarmManager
        println(am)
    }

    // A local class is not the platform manager either; Go misses it for the same reason.
    fun localClassWithManagerName() {
        class PowerManager
        val pm = <!ServiceCast!>context.getSystemService(Context.POWER_SERVICE) as PowerManager<!>
        println(pm)
    }

    // android.text.ClipboardManager is the superclass of android.content.ClipboardManager, so both casts are safe.
    @Suppress("DEPRECATION")
    fun clipboard() {
        val current = context.getSystemService(Context.CLIPBOARD_SERVICE) as android.content.ClipboardManager
        val legacy = context.getSystemService(Context.CLIPBOARD_SERVICE) as android.text.ClipboardManager
        println("$current $legacy")
    }
}
