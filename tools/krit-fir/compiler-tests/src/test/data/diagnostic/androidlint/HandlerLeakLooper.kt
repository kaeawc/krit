// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 28, 33, 36, 48, 52
// The Looper exemption for inner Handler classes, as Go applies it: a primary
// constructor parameter whose type names Looper, referenced in the arguments
// of the `Handler(...)` superclass call.
package test

import android.os.Handler
import android.os.Looper

class Screen {
    inner class Passed(looper: Looper) : Handler(looper)

    inner class PassedProperty(val looper: Looper) : Handler(looper)

    inner class PassedNullable(looper: Looper?) : Handler(looper!!)

    inner class PassedWithCallback(looper: Looper, cb: Handler.Callback) : Handler(looper, cb)

    inner class PassedWrapped(looper: Looper?) : Handler(checkNotNull(looper))

    inner class PassedQualified(looper: android.os.Looper) : android.os.Handler(looper)

    // The main looper is not a constructor parameter.
    <!HandlerLeak!>inner<!> class MainLooper : Handler(Looper.getMainLooper())

    // The Looper parameter is not passed to Handler.
    <!HandlerLeak!>inner<!> class NotPassed(looper: Looper) : Handler() {
        val l = looper
    }

    // A parameter that is not a Looper does not exempt the class.
    <!HandlerLeak!>inner<!> class NotLooper(looper: Any) : Handler(looper as Looper)

    // Go reads only the primary constructor.
    <!HandlerLeak!>inner<!> class Secondary : Handler {
        constructor(looper: Looper) : super(looper)
    }

    // The superclass call must name Handler itself.
    open inner class Base(looper: Looper) : Handler(looper)

    // Go misses this: it does not follow the base class Base to Handler.
    // ViaBase is an inner Handler class the exemption does not cover.
    <!HandlerLeak!>inner<!> class ViaBase(looper: Looper) : Base(looper)

    // Anonymous Handlers are reported even with a Looper, as in Go.
    fun create(looper: Looper): Handler = <!HandlerLeak!>object<!> : Handler(looper) {}

    // Go's tree-sitter reads `$looper` as an interpolated identifier, which it
    // does not count, so the Looper parameter is not passed to Handler.
    <!HandlerLeak!>inner<!> class ShortTemplate(looper: Looper) : Handler(Looper.getMainLooper().also { println("$looper") })

    // `${looper}` holds an ordinary identifier: exempt, as in Go.
    inner class BlockTemplate(looper: Looper) : Handler(Looper.getMainLooper().also { println("${looper}") })
}
