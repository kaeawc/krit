// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: none
package test

import android.content.Context

class Screen {
    fun service(context: Context): String = context.getSystemService("service") as String

    class Context {
        fun getSystemService(name: String): Any = Any()
    }
}
