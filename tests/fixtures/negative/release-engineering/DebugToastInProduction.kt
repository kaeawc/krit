package test

import android.content.Context
import android.widget.Toast

fun showSavedToast(context: Context, message: String) {
    Toast.makeText(context, message, Toast.LENGTH_SHORT).show()
}

fun showDebuggerToast(context: Context) {
    Toast.makeText(context, "debugger attached", Toast.LENGTH_SHORT).show()
}

fun showTestingFixtureToast(context: Context) {
    Toast.makeText(context, "testing-fixture loaded", Toast.LENGTH_SHORT).show()
}

fun showWipeToast(context: Context) {
    Toast.makeText(context, "wipe complete", Toast.LENGTH_SHORT).show()
}
