// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 17, 22, 23, 28, 32, 37, 43, 51, 56, 61, 66, 71, 72
// Positive: a receiver whose type is a type parameter bounded by Context is a
// Context. A project helper declared on such a type parameter is a Context
// helper, as `fun Context.grantUriPermissions` is, and each of its bounds is
// tested, in plain, nullable, definitely-not-null and multi-bound spellings.
package test

import android.app.Activity
import android.content.Context
import android.content.Intent
import android.net.Uri

private const val FLAG = Intent.FLAG_GRANT_READ_URI_PERMISSION

fun <T : Context> T.grantUriPermissions(uri: Uri) {
    <!GrantAllUris!>grantUriPermission("com.other", uri, FLAG)<!>
}

class Screen : Activity() {
    fun go(uri: Uri) {
        <!GrantAllUris!>grantUriPermissions(uri)<!>
        <!GrantAllUris!>this.grantUriPermissions(uri)<!>
    }
}

fun onActivity(activity: Activity, uri: Uri) {
    <!GrantAllUris!>activity.grantUriPermissions(uri)<!>
}

fun onContext(context: Context, uri: Uri) {
    <!GrantAllUris!>context.grantUriPermissions(uri)<!>
}

// The receiver of the unqualified call is `this`, typed as the type parameter.
fun <T : Context> T.shareFirst(uris: List<Uri>) {
    <!GrantAllUris!>grantUriPermissions(uris.first())<!>
}

// Several bounds, one of them a Context.
fun <A> A.grantUriPermissions(uris: List<Uri>) where A : Comparable<A>, A : Context {
    for (uri in uris) {
        <!GrantAllUris!>grantUriPermission("com.other", uri, FLAG)<!>
    }
}

class SortedActivity : Activity(), Comparable<SortedActivity> {
    override fun compareTo(other: SortedActivity): Int = 0

    fun go(uris: List<Uri>) {
        <!GrantAllUris!>grantUriPermissions(uris)<!>
    }
}

fun onSorted(activity: SortedActivity, uris: List<Uri>) {
    <!GrantAllUris!>activity.grantUriPermissions(uris)<!>
}

// A nullable bound and a definitely-not-null receiver.
fun <T : Context?> T.grantUriPermissions(uri: Uri, modeFlags: Int) {
    <!GrantAllUris!>this?.grantUriPermission("com.other", uri, modeFlags)<!>
}

fun <T : Context?> (T & Any).grantUriPermissions(uris: Set<Uri>) {
    for (uri in uris) {
        <!GrantAllUris!>grantUriPermission("com.other", uri, FLAG)<!>
    }
}

fun onNullableBounds(maybe: Context?, context: Context, uri: Uri) {
    <!GrantAllUris!>maybe.grantUriPermissions(uri, FLAG)<!>
    <!GrantAllUris!>context.grantUriPermissions(setOf(uri))<!>
}

// Deliberate improvement: Go misses this. Its source inference types the
// receiver as the type parameter T without its bound, so it concludes the
// receiver is not a Context; T is bounded by Context, so this is a URI
// permission grant on a Context.
fun <T : Context> share(t: T, uri: Uri) {
    <!GrantAllUris!>t.grantUriPermission("com.other", uri, FLAG)<!>
}
