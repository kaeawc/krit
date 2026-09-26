// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 36, 42
// Unfinalized SharedPreferences.edit() calls Go misses: it only searches
// function bodies, and it matches the editor variable by name.
package test

import android.content.SharedPreferences

class Recall(prefs: SharedPreferences) {
    // Go finds no enclosing function around an init block or a secondary
    // constructor body, so it reports nothing there.
    init {
        val editor = <!CommitPrefEdits!>prefs.edit()<!>
        editor.putString("k", "v")
    }

    constructor(prefs: SharedPreferences, key: String) : this(prefs) {
        <!CommitPrefEdits!>prefs.edit()<!>.remove(key)
    }
}

class Shadowed(private val prefs: SharedPreferences) {
    // Go matches `editor.apply()` by name; that call applies the lambda's own
    // editor parameter, not the one this edit call returned.
    fun shadowedName(others: List<SharedPreferences.Editor>) {
        val editor = <!CommitPrefEdits!>prefs.edit()<!>
        editor.putString("k", "v")
        others.forEach { editor -> editor.apply() }
    }
}

class ScopeNegatives(private val prefs: SharedPreferences) {
    // Matching Go: an apply() in a nested lambda may never run, and an apply()
    // on another editor inside the scope lambda does not finalize this one.
    fun nestedLambda(items: List<String>) {
        <!CommitPrefEdits!>prefs.edit()<!>.apply {
            items.forEach { apply() }
        }
    }

    fun otherEditor(other: SharedPreferences.Editor) {
        <!CommitPrefEdits!>prefs.edit()<!>.also { other.apply() }
    }
}
