// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 25
package test

import android.content.Context
import android.os.PowerManager

// A project type that has its own getSystemService is not the platform lookup.
class Registry {
    fun getSystemService(name: String): Any = name
}

// A constant that only shares the platform constant's name does not name the platform service.
object Names {
    const val ALARM_SERVICE = "alarm"
}

class Lookalikes(private val context: Context, private val registry: Registry) {
    fun localLookup() {
        val pm = registry.getSystemService(Context.ALARM_SERVICE) as PowerManager
        println(pm)
    }

    fun localConstant() {
        val pm = context.getSystemService(Names.ALARM_SERVICE) as PowerManager
        println(pm)
    }
}
