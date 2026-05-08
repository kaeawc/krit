package test

import android.content.ContentResolver
import android.net.Uri

class UserLookup {
    fun load(resolver: ContentResolver, uri: Uri, name: String) {
        resolver.query(uri, null, "name = '$name'", null, null)
    }
}
