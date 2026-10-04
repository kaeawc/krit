// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 22, 23, 49, 57, 65
// Negative: functions named grantUriPermission(s) that are not members of, or
// extensions on, a Context. None of them grants a URI permission through the
// Android Context API, so none is reported. Go reports the ones listed in the
// go-lines header, each marked below: it reports every unqualified call with
// that name, and a qualified one whose receiver it cannot type.
package test

import android.content.Context
import android.content.Intent
import android.net.Uri

class Grants {
    fun grantUriPermission(toPackage: String, uri: Uri, modeFlags: Int) {}

    fun grantUriPermissions(toPackage: String, uris: List<Uri>) {}

    // Go reports both calls: an unqualified call is always a finding there.
    // They resolve to the Grants members above.
    fun forward(uri: Uri) {
        grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
        grantUriPermissions("com.other", listOf(uri))
    }
}

fun makeGrants(): Grants = Grants()

object GrantsRegistry {
    val shared: Grants by lazy { Grants() }
    var maybe: Grants? = null
}

fun onTypedLookalike(grants: Grants, all: List<Grants>, any: Any, uri: Uri) {
    grants.grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    grants.grantUriPermissions("com.other", listOf(uri))
    makeGrants().grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    Grants().grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    all.first().grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    (all.firstOrNull() ?: makeGrants()).grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    (any as Grants).grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    GrantsRegistry.shared.grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    GrantsRegistry.maybe?.grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
}

// Go reports this: it cannot type the `!!` receiver, and an untyped receiver
// falls back to a finding.
fun onUntypedLookalike(uri: Uri) {
    GrantsRegistry.maybe!!.grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
}

fun grantUriPermission(toPackage: String, uri: Uri) {}

// Go reports this: an unqualified call is always a finding there, and here it
// resolves to the top-level function above, even inside a Context extension.
fun Context.topLevelLookalike(uri: Uri) {
    grantUriPermission("com.other", uri)
}

class Holder(val grantUriPermission: (String, Uri, Int) -> Unit)

// Go reports this: an invoked function-typed property is an unqualified call
// named grantUriPermission.
fun Holder.invokeProperty(uri: Uri) {
    grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
}

fun otherContextApis(context: Context, uri: Uri) {
    // context.grantUriPermission(pkg, uri, flags) in a comment
    context.revokeUriPermission(uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    val reference = context::grantUriPermission
    reference("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    val text = "context.grantUriPermission(pkg, uri, flags)"
    text.length
}
