// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 13, 15, 18
// Positives: import directives of the constant split across lines. Go reports
// the MODE_WORLD_WRITEABLE identifier, so FIR reports each directive on the
// imported name's line, not on the line of the `import` keyword; an alias
// after the name does not move it. Divergence (recall), as in
// WorldWriteableFilesImportAlias.kt: Go misses the read through the alias WW,
// so the last line gets one Go finding and two FIR findings.
package test

import android.content.Context
import android.content.Context
    .<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!>
import android.content.Context
    .<!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> as WW
import java.io.FileOutputStream

fun split(context: Context): FileOutputStream = context.openFileOutput("a.txt", <!WorldWriteableFiles!>MODE_WORLD_WRITEABLE<!> or <!WorldWriteableFiles!>WW<!>)
