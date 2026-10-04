// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 14, 18, 23, 28
// Positive: a Context reached through a typealias or an import alias is still
// android.content.Context. Resolution sees through both spellings.
package test

import android.content.Context as AndroidContext
import android.content.Intent
import android.net.Uri

typealias Ctx = AndroidContext

fun onTypealias(context: Ctx, uri: Uri) {
    <!GrantAllUris!>context.grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)<!>
}

fun onImportAlias(context: AndroidContext, uri: Uri) {
    <!GrantAllUris!>context.grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)<!>
}

fun Ctx.grantUriPermissions(uris: List<Uri>) {
    for (uri in uris) {
        <!GrantAllUris!>grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)<!>
    }
}

fun onTypealiasExtension(context: AndroidContext, uris: List<Uri>) {
    <!GrantAllUris!>context.grantUriPermissions(uris)<!>
}
