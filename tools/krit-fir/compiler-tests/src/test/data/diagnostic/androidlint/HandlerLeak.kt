// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 16, 21, 24, 34, 54, 57, 62
// Inner Handler classes and anonymous Handler objects, as Lint and Go report them.
package test

import android.os.Handler
import android.os.Looper
import android.os.Message

class Screen {
    <!HandlerLeak!>inner<!> class MyHandler : Handler() {
        override fun handleMessage(msg: Message) {}
    }

    /** KDoc belongs to the class in the light tree but not in Go's node. */
    <!HandlerLeak!>@Suppress("DEPRECATION")<!>
    inner class Annotated : Handler() {
        override fun handleMessage(msg: Message) {}
    }

    <!HandlerLeak!>private<!>
    inner class SplitModifiers : Handler()

    val anonymous = <!HandlerLeak!>object<!> : Handler() {
        override fun handleMessage(msg: Message) {}
    }

    // Built on an explicit Looper: Lint and Go skip it.
    fun inFunction() {
        val h = object : Handler(Looper.getMainLooper()) {}
        h.sendEmptyMessage(1)
    }

    fun inLambda(): () -> Handler = { <!HandlerLeak!>object<!> : Handler() {} }

    // A nested (non-inner) class holds no reference to Screen.
    class Nested : Handler()

    // Not a Handler.
    inner class Worker : Runnable {
        override fun run() {}
    }

    inner class CallbackImpl : Handler.Callback {
        override fun handleMessage(msg: Message): Boolean = true
    }

    val runnable = object : Runnable {
        override fun run() {}
    }
}

// Anonymous Handlers are reported wherever they are declared, as in Lint and Go.
val topLevel = <!HandlerLeak!>object<!> : Handler() {}

object Holder {
    val h = <!HandlerLeak!>object<!> : Handler() {}
}

class WithCompanion {
    companion object {
        val h = <!HandlerLeak!>object<!> : Handler() {}
    }
}
