// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 42, 46, 71
// Anonymous functions (`fun() { }`) around the edit call or the finalizing
// call. Go's enclosing-call walk and its later-call search cross anonymous
// functions and lambdas alike, and so do the checker's.
package test.anonymous

import android.content.SharedPreferences

class Deferred(private val block: () -> Unit) {
    fun commit() = block()
}

class Anonymous(private val prefs: SharedPreferences) {
    // A no-argument commit() that encloses an anonymous function (or a
    // lambda) holding the edit call. Go accepts any enclosing call named
    // commit or apply, by name, up to the named function, and does not report
    // these; the checker mirrors that walk, so neither reports them. The
    // Editor itself is never committed: this is a miss both share, not a
    // divergence.
    fun enclosedAnonymous() {
        Deferred(fun() { prefs.edit().putString("k", "v") }).commit()
    }

    fun enclosedLambda() {
        Deferred { prefs.edit().putString("k", "v") }.commit()
    }

    fun enclosedAnonymousSplit() {
        Deferred(
            fun() {
                prefs.edit().putString("k", "v")
            },
        ).commit()
    }

    // An anonymous function passed to a scope function runs at once, like a
    // lambda, and applies or commits the Editor it receives. Go reports both
    // calls (the finalizing call has an argument, so it is not Go's
    // finalizing shape); the message is false, so the checker does not.
    fun alsoAnonymous() {
        prefs.edit().putString("k", "v").also(fun(e: SharedPreferences.Editor) { e.apply() })
    }

    fun withAnonymousReceiver() {
        with(prefs.edit(), fun SharedPreferences.Editor.() { commit() })
    }

    // A later `editor.apply()` inside an anonymous function or a lambda in the
    // same function finalizes the variable, for Go and the checker.
    fun variableAppliedInAnonymous() {
        val editor = prefs.edit()
        editor.putString("k", "v")
        run(fun() { editor.apply() })
    }

    fun variableAppliedInLambda() {
        val editor = prefs.edit()
        editor.putString("k", "v")
        run { editor.apply() }
    }

    fun deferredApply(): () -> Unit {
        val editor = prefs.edit()
        return fun() { editor.apply() }
    }

    // The scope function's anonymous function only declares another one that
    // would apply the Editor; nothing runs it, so both report the call.
    fun anonymousNotRun() {
        <!CommitPrefEdits!>prefs.edit()<!>.putString("k", "v").also(fun(e: SharedPreferences.Editor) { val later = fun() { e.apply() } })
    }
}
