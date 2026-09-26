// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 20, 32, 37
// A member extension decides on its extension receiver, not on the class that
// declares it. One on a String or a Uri inside an Activity is not a grant on a
// Context, even though its dispatch receiver is the Activity; one on a Context
// inside a class that is not a Context is. Go decides on the call's receiver
// the same way.
package test

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.Uri

class StrActivity : Activity() {
    fun String.grantUriPermissions() {}

    fun Uri.grantUriPermissions(pkg: String) {
        // The grant inside is on the Activity, its implicit dispatch receiver.
        <!GrantAllUris!>grantUriPermission(pkg, this, Intent.FLAG_GRANT_READ_URI_PERMISSION)<!>
    }

    fun go(uri: Uri) {
        "x".grantUriPermissions()
        uri.grantUriPermissions("com.other")
    }
}

class Sharer {
    fun Context.grantUriPermissions(uris: List<Uri>) {
        for (uri in uris) {
            <!GrantAllUris!>grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)<!>
        }
    }

    fun share(context: Context, uris: List<Uri>) {
        <!GrantAllUris!>context.grantUriPermissions(uris)<!>
    }
}
