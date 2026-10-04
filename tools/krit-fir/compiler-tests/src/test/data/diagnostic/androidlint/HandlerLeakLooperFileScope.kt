// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 23
// The Looper exemption for an inner Handler class without a primary
// constructor. Go finds no primary constructor under the class and walks the
// whole file instead, so it takes the Looper parameters of every primary
// constructor in the file; FIR matches it.
package test

import android.os.Handler
import android.os.Looper

class Holder(val looper: Looper)

class Worker(private val looper: Looper) {
    // Built with the outer class's caller-supplied Looper: exempt, as in Go.
    private inner class WorkHandler : Handler(looper)

    // An empty primary constructor holds no Looper parameter, so the file is
    // not searched and the class is reported, as in Go.
    <!HandlerLeak!>inner<!> class EmptyConstructor() : Handler(looper)

    // No Looper parameter anywhere in the file is named mainLooper.
    <!HandlerLeak!>inner<!> class MainLooper : Handler(Looper.getMainLooper())
}

class Screen(private val src: Holder) {
    // Holder's `looper: Looper` parameter is spelled like the argument's
    // selector: exempt, as in Go.
    inner class ViaProp : Handler(src.looper)
}
