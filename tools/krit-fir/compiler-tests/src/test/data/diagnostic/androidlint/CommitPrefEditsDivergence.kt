// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 24, 25, 30, 35, 41, 47, 53, 57, 66, 71, 79, 89
// Go findings the checker drops because the message is false of the code: the
// call is not SharedPreferences.edit(), or the Editor it returns is committed
// or applied. Go reports every one of these.
package test

import android.content.SharedPreferences
import androidx.core.content.edit

class Document {
    fun edit(): Document = this
    fun save() {}
}

object Settings {
    fun edit() {}
}

class Divergence(private val prefs: SharedPreferences, private val doc: Document) {
    // Go matches any zero-argument call named edit; neither is a
    // SharedPreferences, so no editor is left open.
    fun localLookalikes() {
        doc.edit().save()
        Settings.edit()
    }

    // The androidx.core KTX edit { } applies the Editor itself after the block.
    fun ktxEdit(key: String) {
        prefs.edit { putString(key, "v") }
    }

    // The scope function's lambda applies the Editor through its receiver.
    fun scopeApplyThenApplies(key: String) {
        prefs.edit().apply {
            putString(key, "v")
            apply()
        }
    }

    fun runCommits(key: String): Boolean = prefs.edit().run {
        remove(key)
        commit()
    }

    fun withCommits(key: String) {
        with(prefs.edit()) {
            putString(key, "v").commit()
        }
    }

    fun alsoApplies(key: String) {
        prefs.edit().putString(key, "v").also { it.apply() }
    }

    fun letApplies(key: String) {
        prefs.edit().let { editor ->
            editor.remove(key)
            editor.apply()
        }
    }

    // Go only reads a finalizing call written `editor.apply()`, not one at
    // the end of a chain on the variable.
    fun variableChainApplies(key: String) {
        val editor = prefs.edit()
        editor.putString(key, "v").apply()
    }

    fun variableScopeApplies(key: String) {
        val editor = prefs.edit()
        editor.apply {
            putString(key, "v")
            commit()
        }
    }

    fun variableWithApplies(key: String) {
        val editor = prefs.edit()
        with(editor) {
            remove(key)
            apply()
        }
    }

    // Go only reads val/var initializers, not an assignment.
    fun assignedThenApplied(key: String) {
        val editor: SharedPreferences.Editor
        editor = prefs.edit()
        editor.remove(key)
        editor.apply()
    }
}
