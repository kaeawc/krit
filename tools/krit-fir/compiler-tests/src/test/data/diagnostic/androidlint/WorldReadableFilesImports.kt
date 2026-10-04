// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 11, 12, 14
// Positives: a direct import of Context.MODE_WORLD_READABLE, an import alias
// of it, and their uses. Go reports each import line and the bare use by the
// identifier's text, so FIR reports the import directives too.
// Divergence (recall): the aliased use `WR` is the platform constant, but Go
// only matches the text MODE_WORLD_READABLE and misses it; FIR resolves it.
package test

import android.content.Context
import android.content.Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!>
import android.content.Context.<!WorldReadableFiles!>MODE_WORLD_READABLE<!> as WR

fun imported(context: Context) = context.getSharedPreferences("data", <!WorldReadableFiles!>MODE_WORLD_READABLE<!>)

fun importAlias(context: Context) = context.getSharedPreferences("data", <!WorldReadableFiles!>WR<!>)
