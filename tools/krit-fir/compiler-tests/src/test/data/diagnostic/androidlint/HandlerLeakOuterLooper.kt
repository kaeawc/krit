// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 18, 20, 24
// An inner Handler class built on a Looper it does not take itself: from an
// outer class's property, or another object's. Lint 32.4.1 reads only the
// class's own constructor parameters, so it reports each one, and so do Go and
// FIR. (Go used to take the Looper parameters of every primary constructor in
// the file when the class had none of its own.)
package test

import android.os.Handler
import android.os.Looper

class Holder(val looper: Looper)

class Worker(private val looper: Looper) {
    <!HandlerLeak!>private<!> inner class WorkHandler : Handler(looper)

    <!HandlerLeak!>inner<!> class EmptyConstructor() : Handler(looper)

    <!HandlerLeak!>inner<!> class MainLooper : Handler(Looper.getMainLooper())
}

class Screen(private val src: Holder) {
    <!HandlerLeak!>inner<!> class ViaProp : Handler(src.looper)
}
