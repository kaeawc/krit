package com.example

import android.content.Context
import android.net.Uri

class SafeProvider {
    fun share(context: Context, uri: Uri) {
        context.contentResolver.query(uri, null, null, null, null)
    }
}
