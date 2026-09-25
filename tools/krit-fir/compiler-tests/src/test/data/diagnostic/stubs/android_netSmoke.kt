// Smoke: Uri.parse (a Java static) and the Uri.Builder chain.
package stubs

import android.net.Uri

fun buildUri(): Uri {
    val parsed: Uri = Uri.parse("https://example.com/path?q=1")
    val query: String? = parsed.getQueryParameter("q")
    val scheme: String? = parsed.scheme
    <!PrintlnInProduction!>println<!>("$scheme ${parsed.host} ${parsed.path} ${parsed.lastPathSegment} ${Uri.EMPTY}")
    return parsed.buildUpon()
        .appendPath("child")
        .appendQueryParameter("q", query)
        .build()
}

fun freshUri(): Uri = Uri.Builder().scheme("https").authority("example.com").build()
