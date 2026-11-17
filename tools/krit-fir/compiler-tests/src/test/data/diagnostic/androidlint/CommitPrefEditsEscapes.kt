// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 52, 63, 68
// Editors that escape the function, which Lint 32.4.1 and Go leave alone: the
// edit chain or the variable holding it is returned, passed as an argument, or
// assigned to a variable that is not local. Its new owner may commit it.
package test.escapes

import android.content.SharedPreferences

fun consume(editor: SharedPreferences.Editor) {}

class Escapes(private val prefs: SharedPreferences) {
    private var field: SharedPreferences.Editor? = null

    fun expressionBody(): SharedPreferences.Editor = prefs.edit()

    fun returnedChain(): SharedPreferences.Editor = prefs.edit().putString("k", "v")

    fun storedInField() {
        field = prefs.edit()
    }

    fun passedChain() {
        consume(prefs.edit().remove("k"))
    }

    fun collected(): List<SharedPreferences.Editor> = listOf(prefs.edit())

    fun variableReturned(): SharedPreferences.Editor {
        val editor = prefs.edit()
        editor.putString("k", "v")
        return editor
    }

    fun variablePassed() {
        val editor = prefs.edit()
        editor.putString("k", "v")
        consume(editor)
    }

    fun variableStored() {
        val editor = prefs.edit()
        field = editor
    }

    fun variablePassedFromLambda(items: List<String>) {
        val editor = prefs.edit()
        items.forEach { consume(editor) }
    }

    // Still reported: none of these hands the Editor on.
    fun lambdaResult(items: List<String>): List<Unit> = items.map { <!CommitPrefEdits!>prefs.edit()<!>.remove(it); Unit }

    // Go reads any assignment as handing the Editor on; this one only fills
    // a local variable that is never finalized.
    fun localAssignment() {
        var editor: SharedPreferences.Editor
        editor = <!CommitPrefEdits!>prefs.edit()<!>
        editor.remove("k")
    }

    fun variableUsedAsReceiver() {
        val editor = <!CommitPrefEdits!>prefs.edit()<!>
        editor.remove("k").clear()
    }

    fun variableReadInTemplate() {
        val editor = <!CommitPrefEdits!>prefs.edit()<!>
        println("$editor")
    }
}
