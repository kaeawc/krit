// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11
// Positive: a star import of Context's statics. The bare name resolves to the
// platform constant, and Go reports the identifier by name, so both report the
// use. The star import itself names no constant and is reported by neither.
package test

import android.content.Context
import android.content.Context.*

fun starImported(context: Context) = context.getSharedPreferences("data", <!WorldReadableFiles!>MODE_WORLD_READABLE<!>)
