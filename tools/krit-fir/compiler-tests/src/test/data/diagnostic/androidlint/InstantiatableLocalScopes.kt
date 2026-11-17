// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 19, 23, 29, 38, 43, 45, 48
// Components declared in local scopes: members of an `object { ... }`
// expression and of a local class. FIR gives a class declared `private` there
// Local visibility, but the class still carries the `private` modifier Go
// reads, so it is reported as a private class.
package test.local

import android.app.Activity
import android.app.Service
import android.content.Intent
import android.os.IBinder

// A private inner class of a top-level object expression.
val top = object {
    <!Instantiatable!>private<!> inner class PrivateInTopObject : Activity()

    // A private primary constructor in an object expression.
    <!Instantiatable!>inner<!> class PrivateCtorInTopObject private constructor() : Activity()

    // An inner class, even a public one with a public constructor: its
    // constructor takes the outer instance, as Android Lint reports.
    <!Instantiatable!>inner<!> class PublicInTopObject : Activity()
}

class Host {
    // A private inner class of an object expression held by a member.
    private val holder = object {
        <!Instantiatable!>private<!> inner class PrivateInMemberObject : Service() {
            override fun onBind(intent: Intent?): IBinder? = null
        }
    }
}

fun localScopes() {
    // A private inner class of an object expression inside a function.
    val inFunction = object {
        <!Instantiatable!>private<!> inner class PrivateInFunctionObject : Activity()
    }

    // A private inner class of a local class.
    class LocalHost {
        <!Instantiatable!>private<!> inner class PrivateInLocalClass : Activity()

        <!Instantiatable!>inner<!> class PrivateCtorInLocalClass private constructor() : Activity()

        // A public inner class of a local class.
        <!Instantiatable!>inner<!> class PublicInLocalClass : Activity()
    }
}
