package com.example
import android.content.Context
import android.content.Intent
import android.net.Uri
class MyProvider(base: Context) : android.content.ContextWrapper(base) {
    fun share(context: Context, uri: Uri) {
        context.grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }

    fun shareBroad(context: Context) {
        grantUriPermission("com.pkg", Uri.parse("content://authority/"), Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
}
