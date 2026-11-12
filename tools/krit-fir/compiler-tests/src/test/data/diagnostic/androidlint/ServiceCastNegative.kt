// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import android.content.Context
import android.net.ConnectivityManager
import android.net.wifi.WifiManager
import android.os.PowerManager

class Monitor(private val context: Context) {
    fun correct() {
        val wm = context.getSystemService(Context.WIFI_SERVICE) as WifiManager
        val cm = context.getSystemService(Context.CONNECTIVITY_SERVICE) as ConnectivityManager
        val pm = context.getSystemService(Context.POWER_SERVICE) as? PowerManager
        println("$wm $cm $pm")
    }

    fun stringLiteralName() {
        val pm = context.getSystemService("alarm") as PowerManager
        println(pm)
    }

    fun classOverload() {
        val pm = context.getSystemService(PowerManager::class.java)
        println(pm)
    }

    fun storedObject() {
        val service = context.getSystemService(Context.ALARM_SERVICE)
        val pm = service as PowerManager
        println(pm)
    }

    fun notAServiceCall(lookup: Lookup) {
        val pm = lookup.find(Context.ALARM_SERVICE) as PowerManager
        println(pm)
    }
}

class Lookup {
    fun find(name: String): Any = name
}
