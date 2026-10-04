// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 13, 18, 22, 24
// Supertypes seen through `import android.os.*`.
package test

import android.os.*

open class LocalBase

class Screen {
    <!HandlerLeak!>inner<!> class Real : Handler()

    val real = <!HandlerLeak!>object<!> : Handler() {}

    // Go reports these because, with `import android.os.*`, it takes any
    // supertype name that no explicit import names for android.os.Handler;
    // none of them is a Handler.
    inner class Worker : Runnable {
        override fun run() {}
    }

    inner class Derived : LocalBase()

    val runnable = object : Runnable {
        override fun run() {}
    }
}
