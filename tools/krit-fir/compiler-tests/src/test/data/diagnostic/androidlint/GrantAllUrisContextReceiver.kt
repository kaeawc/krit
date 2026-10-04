// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 25, 34, 35, 40, 49, 58, 66, 71, 72, 76
// Positive, matching Go: a call named grantUriPermission(s) whose receiver is a
// Context is reported even when the function is declared on something that is
// not a Context (an interface a Context subclass implements, an extension on
// Any). Go decides on the receiver's type alone. What the helper does is not
// visible at the call, and a helper by this name called on a Context is how a
// project grants URI permissions (these ones do), so the call is treated as a
// grant on a Context, like a call of `fun Context.grantUriPermissions`.
package test

import android.app.Activity
import android.content.Context
import android.content.ContextWrapper
import android.content.Intent
import android.net.Uri

private const val FLAG = Intent.FLAG_GRANT_READ_URI_PERMISSION

interface UriGranter {
    val grantContext: Context

    fun grantUriPermissions(uris: List<Uri>) {
        for (uri in uris) {
            <!GrantAllUris!>grantContext.grantUriPermission("com.other", uri, FLAG)<!>
        }
    }
}

class MixinActivity : Activity(), UriGranter {
    override val grantContext: Context get() = this

    fun go(uris: List<Uri>) {
        <!GrantAllUris!>grantUriPermissions(uris)<!>
        <!GrantAllUris!>this.grantUriPermissions(uris)<!>
    }
}

fun onMixin(activity: MixinActivity, uris: List<Uri>) {
    <!GrantAllUris!>activity.grantUriPermissions(uris)<!>
}

// The receivers here are a local class and an anonymous object.
fun localMixins(base: Context, uris: List<Uri>) {
    class LocalMixin : ContextWrapper(base), UriGranter {
        override val grantContext: Context get() = this

        fun go() {
            <!GrantAllUris!>grantUriPermissions(uris)<!>
        }
    }
    LocalMixin().go()

    val anonymous = object : ContextWrapper(base), UriGranter {
        override val grantContext: Context get() = this

        fun go() {
            <!GrantAllUris!>grantUriPermissions(uris)<!>
        }
    }
    anonymous.go()
}

fun Any.grantUriPermissions(pkg: String, uri: Uri) {
    if (this is Context) {
        <!GrantAllUris!>grantUriPermission(pkg, uri, FLAG)<!>
    }
}

fun onAnyExtension(context: Context, activity: Activity, any: Any, uri: Uri) {
    <!GrantAllUris!>context.grantUriPermissions("com.other", uri)<!>
    <!GrantAllUris!>activity.grantUriPermissions("com.other", uri)<!>
    // The receiver is not a Context.
    any.grantUriPermissions("com.other", uri)
    if (any is Context) {
        <!GrantAllUris!>any.grantUriPermissions("com.other", uri)<!>
    }
}
