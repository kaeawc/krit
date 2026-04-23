package com.example

import android.content.Context

class MyProvider {
    fun share(context: Context) {
        context.grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }

    fun shareBroad(context: Context) {
        grantUriPermission("com.pkg", Uri.parse("content://authority/"), Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
}
