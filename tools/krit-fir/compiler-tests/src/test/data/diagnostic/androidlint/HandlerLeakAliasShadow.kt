// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 15, 24, 27
// android.os.Handler imported under an alias while a local class takes the
// simple name Handler.
package test

import android.os.Handler as AndroidHandler
import android.os.Looper as AndroidLooper

open class Handler

typealias LooperAlias = android.os.Looper

class Screen {
    <!HandlerLeak!>inner<!> class Real : AndroidHandler()

    inner class Local : Handler()

    val local = object : Handler() {}

    // Go reads the parameter type as written, so it does not see that
    // AndroidLooper is android.os.Looper and reports this. The constructor
    // takes a Looper, which Lint exempts, so it is not reported here.
    inner class AliasedLooper(looper: AndroidLooper) : AndroidHandler(looper)

    // A type alias of Looper, likewise.
    inner class TypeAliasedLooper(looper: LooperAlias) : AndroidHandler(looper)
}
