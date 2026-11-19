// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 22, 31
// Wrong casts reached through an inherited constant, a subclass qualifier, an
// implicit receiver, and a parenthesized argument. Android Lint reports each
// of them. Go misses only the parenthesized constant (recall): it reads the
// argument as written.
package test.servicecast.recall

import android.app.Activity
import android.content.Context
import android.os.PowerManager

class Screen : Activity() {
    // The constant Context declares, read without a qualifier inside a subclass.
    fun inheritedConstant() {
        val pm = <!ServiceCast!>getSystemService(ALARM_SERVICE) as PowerManager<!>
        println(pm)
    }

    // The constant read through the subclass.
    fun constantThroughSubclass() {
        val pm = <!ServiceCast!>getSystemService(Activity.ALARM_SERVICE) as PowerManager<!>
        println(pm)
    }
}

class Implicit(private val context: Context) {
    // An implicit receiver from a scope function.
    fun scopeFunction() {
        with(context) {
            val pm = <!ServiceCast!>getSystemService(Context.ALARM_SERVICE) as PowerManager<!>
            println(pm)
        }
    }

    // A parenthesized constant argument.
    fun parenthesizedConstant() {
        val pm = <!ServiceCast!>context.getSystemService((Context.ALARM_SERVICE)) as PowerManager<!>
        println(pm)
    }
}
