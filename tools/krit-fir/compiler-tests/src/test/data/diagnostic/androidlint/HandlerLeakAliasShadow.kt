// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 21
// android.os.Handler imported under an alias while a local class takes the
// simple name Handler.
package test

import android.os.Handler as AndroidHandler
import android.os.Looper as AndroidLooper

open class Handler

class Screen {
    <!HandlerLeak!>inner<!> class Real : AndroidHandler()

    inner class Local : Handler()

    val local = object : Handler() {}

    // As in Go, the exemption reads the names as written, so aliases do not
    // count.
    <!HandlerLeak!>inner<!> class AliasedLooper(looper: AndroidLooper) : AndroidHandler(looper)
}
