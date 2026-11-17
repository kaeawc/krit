// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 26, 34, 48, 50, 63
// The Looper exemptions, as in Lint 32.4.1 (HandlerDetector) and Go: an inner
// Handler class one of whose own constructors takes a parameter typed Looper,
// whether or not it passes it to Handler, and an anonymous Handler whose
// superclass constructor call has a Looper argument.
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

    // Lint only asks for a Looper parameter, not that it reaches Handler.
    inner class NotPassed(looper: Looper) : Handler() {
        val l = looper
    }

    // A parameter that is not a Looper does not exempt the class.
    <!HandlerLeak!>inner<!> class NotLooper(looper: Any) : Handler(looper as Looper)

    // A secondary constructor's Looper parameter counts too.
    inner class Secondary : Handler {
        constructor(looper: Looper) : super(looper)
    }

    open inner class Base(looper: Looper) : Handler(looper)

    // The exemption reads the class's own constructors, whatever its
    // superclass; Go does not follow Base to Handler at all.
    inner class ViaBase(looper: Looper) : Base(looper)

    // Neither a list of Loopers nor a vararg is a Looper parameter.
    <!HandlerLeak!>inner<!> class LooperList(loopers: List<Looper>) : Handler(loopers.first())

    <!HandlerLeak!>inner<!> class LooperVararg(vararg loopers: Looper) : Handler(loopers.first())

    // An anonymous Handler built on a Looper argument.
    fun create(looper: Looper): Handler = object : Handler(looper) {}

    fun createMain(): Handler = object : Handler(Looper.getMainLooper()) {}

    fun createWithCallback(cb: Handler.Callback): Handler = object : Handler(Looper.getMainLooper(), cb) {}

    // `l` does not name a looper; Go's resolver types the parameter instead.
    fun createShortName(l: Looper): Handler = object : Handler(l) {}

    // A callback is not a Looper.
    fun createCallbackOnly(cb: Handler.Callback): Handler = <!HandlerLeak!>object<!> : Handler(cb) {}
}
