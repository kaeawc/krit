// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 16, 21, 25, 29, 34, 40, 47, 54, 58, 62, 66, 71, 77, 80, 83, 87, 92, 97, 106
// SharedPreferences.edit() calls whose Editor is never committed or applied,
// the shape the Go rule reports, in every kind of function container, and the
// finalized forms Go leaves alone.
package test

import android.content.SharedPreferences
import androidx.core.content.edit
import androidx.security.crypto.EncryptedSharedPreferences

fun consume(editor: SharedPreferences.Editor) {}

class Prefs(private val prefs: SharedPreferences, private val nullable: SharedPreferences?) {
    fun unused(key: String, value: String) {
        val editor = <!CommitPrefEdits!>prefs.edit()<!>
        editor.putString(key, value)
    }

    fun statement() {
        <!CommitPrefEdits!>prefs.edit()<!>
    }

    fun chainWithoutFinish(key: String) {
        <!CommitPrefEdits!>prefs.edit()<!>.remove(key).clear()
    }

    fun safeCall(key: String) {
        <!CommitPrefEdits!>nullable?.edit()<!>?.remove(key)
    }

    fun implicitReceiver() {
        with(prefs) {
            <!CommitPrefEdits!>edit()<!>
        }
    }

    // Kotlin's scope apply does not persist the edits.
    fun scopeApplyOnly(key: String) {
        <!CommitPrefEdits!>prefs.edit()<!>.apply {
            putString(key, "v")
        }
    }

    // A commit on another editor does not finalize this one.
    fun otherEditorCommitted(other: SharedPreferences.Editor) {
        val editor = <!CommitPrefEdits!>prefs.edit()<!>
        other.commit()
    }

    // An apply before the edit call does not finalize it.
    fun appliedBefore(editor: SharedPreferences.Editor) {
        editor.apply()
        val next = <!CommitPrefEdits!>prefs.edit()<!>
    }

    fun returned(): SharedPreferences.Editor {
        return <!CommitPrefEdits!>prefs.edit()<!>
    }

    fun passed() {
        consume(<!CommitPrefEdits!>prefs.edit()<!>)
    }

    fun inLambda(items: List<String>) {
        items.forEach { <!CommitPrefEdits!>prefs.edit()<!>.remove(it) }
    }

    fun localFunction() {
        fun inner() {
            <!CommitPrefEdits!>prefs.edit()<!>
        }
        inner()
    }

    fun anonymousFunction() = fun() {
        <!CommitPrefEdits!>prefs.edit()<!>
    }

    fun defaultValue(editor: SharedPreferences.Editor = <!CommitPrefEdits!>prefs.edit()<!>) {}

    val getter: SharedPreferences.Editor
        get() = <!CommitPrefEdits!>prefs.edit()<!>

    fun objectExpression() = object : Runnable {
        override fun run() {
            <!CommitPrefEdits!>prefs.edit()<!>
        }
    }

    fun encrypted() {
        <!CommitPrefEdits!>EncryptedSharedPreferences.edit()<!>
    }

    companion object {
        fun fromCompanion(prefs: SharedPreferences) {
            <!CommitPrefEdits!>prefs.edit()<!>
        }
    }
}

interface PrefsOwner {
    val prefs: SharedPreferences

    fun reset() {
        <!CommitPrefEdits!>prefs.edit()<!>.clear()
    }
}

class Finalized(private val prefs: SharedPreferences, private val nullable: SharedPreferences?) {
    fun chainApply(key: String) {
        prefs.edit().putString(key, "v").apply()
    }

    fun chainCommit(key: String): Boolean = prefs.edit().remove(key).commit()

    fun splitChainApply(key: String) {
        prefs
            .edit()
            .putString(key, "v")
            .apply()
    }

    fun safeChain(key: String) {
        nullable?.edit()?.remove(key)?.apply()
    }

    fun variableApply(key: String) {
        val editor = prefs.edit()
        editor.putString(key, "v")
        editor.apply()
    }

    fun variableCommit(key: String) {
        var editor = prefs.edit().putString(key, "v")
        editor.commit()
    }

    fun variableSafeApply(key: String) {
        val editor: SharedPreferences.Editor? = nullable?.edit()
        editor?.remove(key)
        editor?.apply()
    }

    fun variableAppliedInLambda(items: List<String>) {
        val editor = prefs.edit()
        items.forEach { editor.remove(it) }
        items.firstOrNull()?.let { editor.apply() }
    }

    // Like Go, the walk to an enclosing apply() crosses lambdas.
    fun lambdaResultApplied() {
        run { prefs.edit() }.apply()
    }

    fun branchesApplied(flag: Boolean) {
        val editor = if (flag) prefs.edit() else prefs.edit().clear()
        editor.apply()
    }

    fun objectExpressionProperty() {
        val holder = object {
            val editor = prefs.edit()
        }
        holder.editor.apply()
    }

    fun scopeApplyThenApply(key: String) {
        prefs.edit().apply { putString(key, "v") }.apply()
    }

    fun implicitReceiverApply() {
        with(prefs) {
            edit().clear().apply()
        }
    }

    fun encrypted() {
        EncryptedSharedPreferences.edit().clear().commit()
    }

    // Like Go, any enclosing no-argument commit() finalizes the edit call, whatever
    // it is called on: the editor is handed to a wrapper that commits it.
    fun wrapped() {
        Transaction(prefs.edit()).commit()
    }

    // The KTX edit that takes arguments is not SharedPreferences.edit() (Go
    // skips a call with arguments).
    fun ktxWithCommit(key: String) {
        prefs.edit(commit = true) { putString(key, "v") }
    }
}

class Transaction(private val editor: SharedPreferences.Editor) {
    fun commit() {
        editor.commit()
    }
}

// Go reports nothing outside a function body: the editor held in a property
// may be committed by any member.
class Held(prefs: SharedPreferences) {
    val editor = prefs.edit()
    val lazyEditor by lazy { prefs.edit() }
}

val topLevelEditor = EncryptedSharedPreferences.edit()
