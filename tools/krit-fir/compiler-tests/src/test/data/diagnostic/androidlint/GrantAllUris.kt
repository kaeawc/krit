// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 21, 25, 29, 30, 31, 42, 47, 51, 61, 64, 66, 70, 75, 86, 98, 105, 112, 122, 127, 132, 134
// Positive: a URI permission grant on android.content.Context (a Java stub) or
// a subtype, with any receiver shape. Findings sit on the first line of the
// call expression, receiver included. Go reports only some of these (the
// go-lines header); each one it misses is marked below.
package test

import android.app.Activity
import android.app.Application
import android.app.Service
import android.content.Context
import android.content.ContextWrapper
import android.content.Intent
import android.net.Uri

private const val PKG = "com.other"
private const val FLAG = Intent.FLAG_GRANT_READ_URI_PERMISSION

fun onParameter(context: Context, uri: Uri) {
    <!GrantAllUris!>context.grantUriPermission(PKG, uri, FLAG)<!>
}

fun onNullableParameter(context: Context?, uri: Uri) {
    <!GrantAllUris!>context?.grantUriPermission(PKG, uri, FLAG)<!>
}

fun onComponents(activity: Activity, application: Application, service: Service, uri: Uri) {
    <!GrantAllUris!>activity.grantUriPermission(PKG, uri, FLAG)<!>
    <!GrantAllUris!>application.grantUriPermission(PKG, uri, FLAG)<!>
    <!GrantAllUris!>service.grantUriPermission(PKG, uri, FLAG)<!>
}

// Deliberate improvement: Go misses this. Its source inference types the
// parameter as ContextWrapper but has no hierarchy for it, so it concludes
// the receiver is not a Context.
fun onWrapper(wrapper: ContextWrapper, uri: Uri) {
    <!GrantAllUris!>wrapper.grantUriPermission(PKG, uri, FLAG)<!>
}

fun onPropertyChain(context: Context, uri: Uri) {
    <!GrantAllUris!>context.applicationContext.grantUriPermission(PKG, uri, FLAG)<!>
}

fun onLocal(context: Context, uri: Uri) {
    val target = context
    <!GrantAllUris!>target.grantUriPermission(PKG, uri, FLAG)<!>
}

fun multiLine(context: Context, uri: Uri) {
    <!GrantAllUris!>context<!>
        .grantUriPermission(
            PKG,
            uri,
            FLAG,
        )
}

fun scopeFunctions(context: Context, uri: Uri) {
    with(context) {
        <!GrantAllUris!>grantUriPermission(PKG, uri, FLAG)<!>
    }
    context.apply {
        <!GrantAllUris!>grantUriPermission(PKG, uri, FLAG)<!>
    }
    listOf(uri).forEach { <!GrantAllUris!>context.grantUriPermission(PKG, it, FLAG)<!> }
}

fun Context.shareFromExtension(uri: Uri) {
    <!GrantAllUris!>grantUriPermission(PKG, uri, FLAG)<!>
}

class SharingWrapper(base: Context) : ContextWrapper(base) {
    fun implicitReceiver(uri: Uri) {
        <!GrantAllUris!>grantUriPermission(PKG, uri, FLAG)<!>
    }

    // Deliberate improvement: Go misses this. It types `this` as SharingWrapper,
    // whose only supertype, ContextWrapper, has no hierarchy in its source
    // inference.
    fun explicitThis(uri: Uri) {
        <!GrantAllUris!>this.grantUriPermission(PKG, uri, FLAG)<!>
    }

    override fun grantUriPermission(toPackage: String, uri: Uri, modeFlags: Int) {
        <!GrantAllUris!>super.grantUriPermission(toPackage, uri, modeFlags)<!>
    }
}

fun onOverridingSubclass(wrapper: SharingWrapper, uri: Uri) {
    // Deliberate improvement: Go misses this for the reason given on
    // explicitThis.
    <!GrantAllUris!>wrapper.grantUriPermission(PKG, uri, FLAG)<!>
}

class SharingActivity : Activity() {
    fun explicitThis(uri: Uri) {
        <!GrantAllUris!>this.grantUriPermission(PKG, uri, FLAG)<!>
    }
}

fun anonymousAndLocal(base: Context, uri: Uri) {
    val wrapper = object : ContextWrapper(base) {
        fun share() {
            <!GrantAllUris!>grantUriPermission(PKG, uri, FLAG)<!>
        }
    }
    wrapper.share()

    class LocalWrapper : ContextWrapper(base) {
        fun share() {
            <!GrantAllUris!>grantUriPermission(PKG, uri, FLAG)<!>
        }
    }
    LocalWrapper().share()
}

// Go accepts the plural name too. A project helper that grants several URIs on
// a Context is reported.
fun Context.grantUriPermissions(toPackage: String, uris: List<Uri>, modeFlags: Int) {
    for (uri in uris) {
        <!GrantAllUris!>grantUriPermission(toPackage, uri, modeFlags)<!>
    }
}

fun pluralHelper(context: Context, uris: List<Uri>) {
    <!GrantAllUris!>context.grantUriPermissions(PKG, uris, FLAG)<!>
}

fun onCastsAndAssertions(any: Any, maybe: Context?, uri: Uri) {
    if (any is Context) {
        <!GrantAllUris!>any.grantUriPermission(PKG, uri, FLAG)<!>
    }
    <!GrantAllUris!>maybe!!.grantUriPermission(PKG, uri, FLAG)<!>
    // Deliberate improvement: Go misses this. It does not type the
    // parenthesized cast as a Context.
    <!GrantAllUris!>(any as Context).grantUriPermission(PKG, uri, FLAG)<!>
}
