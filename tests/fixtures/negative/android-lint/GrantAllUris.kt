package com.example

import android.content.Context
import android.net.Uri

class SafeProvider {
    // grantUriPermission example — comment should not trigger
    fun share(context: Context, uri: Uri) {
        context.contentResolver.query(uri, null, null, null, null)
        Log.d(TAG, "grantUriPermission called")
    }
}
