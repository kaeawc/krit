// Compiler-test source stubs; never packaged in the production artifact.
package android.net

import java.io.File

// Java abstract class with static factories (Uri.parse) that app code never
// subclasses; modeled as an object so `Uri.parse` keeps its Java callable id.
object Uri {
    val EMPTY: Uri
        get() = TODO()

    val scheme: String?
        get() = TODO()

    val host: String?
        get() = TODO()

    val path: String?
        get() = TODO()

    val lastPathSegment: String?
        get() = TODO()

    val pathSegments: List<String>
        get() = TODO()

    fun parse(uriString: String): Uri = TODO()

    fun fromFile(file: File): Uri = TODO()

    fun encode(s: String?): String? = TODO()

    fun getQueryParameter(key: String): String? = TODO()

    fun buildUpon(): Builder = TODO()

    class Builder {
        fun scheme(scheme: String?): Builder = TODO()

        fun authority(authority: String?): Builder = TODO()

        fun path(path: String?): Builder = TODO()

        fun appendPath(newSegment: String?): Builder = TODO()

        fun appendQueryParameter(key: String?, value: String?): Builder = TODO()

        fun build(): Uri = TODO()
    }
}
