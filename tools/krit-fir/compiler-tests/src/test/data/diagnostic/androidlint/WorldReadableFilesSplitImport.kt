// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 15, 17
// Positives: import directives of the platform constant split across lines.
// Go reports the MODE_WORLD_READABLE identifier, so FIR reports each directive
// on the imported name's line, not on the line of the `import` keyword; an
// alias after the name does not move it. Divergence (recall), as in
// WorldReadableFilesImports.kt: Go misses the use of the alias WR, so the last
// line gets one Go finding and two FIR findings.
package test

import android.content.Context
import android.content.Context
    .<!WorldReadableFiles!>MODE_WORLD_READABLE<!>
import android.content.Context
    .<!WorldReadableFiles!>MODE_WORLD_READABLE<!> as WR

fun split(context: Context) = context.getSharedPreferences("data", <!WorldReadableFiles!>MODE_WORLD_READABLE<!> or <!WorldReadableFiles!>WR<!>)
