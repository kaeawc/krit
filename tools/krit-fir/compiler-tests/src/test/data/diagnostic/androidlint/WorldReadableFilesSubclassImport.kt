// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 9, 12
// Positives: importing the constant through a Context subclass. Activity
// inherits Context's static MODE_WORLD_READABLE, so the import and the use
// name the platform constant; Go reports both by name, and FIR resolves the
// import through Activity's static scope.
package test

<!WorldReadableFiles!>import android.app.Activity.MODE_WORLD_READABLE<!>
import android.content.Context

fun viaSubclassImport(context: Context) = context.getSharedPreferences("data", <!WorldReadableFiles!>MODE_WORLD_READABLE<!>)
