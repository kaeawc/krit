// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 26
package test

import android.app.AlarmManager
import android.content.Context
import android.os.PowerManager

typealias Alarms = AlarmManager

class Divergences(val context: Context) {
    // Go reads the written name Alarms and reports; the alias expands to AlarmManager, the right manager.
    fun aliasOfRightManager() {
        val am = context.getSystemService(Context.ALARM_SERVICE) as Alarms
        println(am)
    }

    // Go reads no target from a nullable type; the cast to PowerManager? is still wrong.
    fun nullableWrongTarget() {
        val pm = <!ServiceCast!>context.getSystemService(Context.ALARM_SERVICE) as PowerManager?<!>
        println(pm)
    }

    // Go reports the type parameter T by name; a type parameter has no class to compare.
    inline fun <reified T> typeParameter() {
        val any = context.getSystemService(Context.ALARM_SERVICE) as T
        println(any)
    }
}
