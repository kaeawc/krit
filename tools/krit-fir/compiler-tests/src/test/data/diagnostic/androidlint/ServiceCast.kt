// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 19, 24, 29, 34, 39, 46, 51, 56
package test

import android.app.Activity
import android.app.AlarmManager
import android.app.NotificationManager
import android.content.Context
import android.net.ConnectivityManager
import android.os.PowerManager

class Monitor(private val context: Context) {
    fun qualified() {
        val cm = <!ServiceCast!>context.getSystemService(Context.WIFI_SERVICE) as ConnectivityManager<!>
        println(cm)
    }

    fun parenthesized() {
        val pm = <!ServiceCast!>(context.getSystemService(Context.ALARM_SERVICE)) as PowerManager<!>
        println(pm)
    }

    fun safeCast() {
        val pm = <!ServiceCast!>context.getSystemService(Context.ALARM_SERVICE) as? PowerManager<!>
        println(pm)
    }

    fun safeReceiver(other: Context?) {
        val pm = <!ServiceCast!>other?.getSystemService(Context.ALARM_SERVICE) as PowerManager<!>
        println(pm)
    }

    fun fullyQualifiedTarget() {
        val nm = <!ServiceCast!>context.getSystemService(Context.ALARM_SERVICE) as android.app.NotificationManager<!>
        println(nm)
    }

    fun castToAny() {
        val any = <!ServiceCast!>context.getSystemService(Context.ALARM_SERVICE) as Any<!>
        println(any)
    }
}

class Screen : Activity() {
    fun bareConstant() {
        val nm = <!ServiceCast!>getSystemService(POWER_SERVICE) as NotificationManager<!>
        println(nm)
    }

    fun thisReceiver() {
        val am = <!ServiceCast!>this.getSystemService(Context.POWER_SERVICE) as AlarmManager<!>
        println(am)
    }

    fun inheritedConstant() {
        val am = <!ServiceCast!>getSystemService(Activity.POWER_SERVICE) as AlarmManager<!>
        println(am)
    }
}
