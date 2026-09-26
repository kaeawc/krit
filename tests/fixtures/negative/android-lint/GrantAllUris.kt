package com.example

import android.content.Context
import android.net.Uri
import android.util.Log

private const val TAG = "SafeProvider"

class SafeProvider {
    // grantUriPermission example — comment should not trigger
    fun share(context: Context, uri: Uri) {
        context.contentResolver.query(uri, null, null, null, null)
        Log.d(TAG, "grantUriPermission called")
    }
}
