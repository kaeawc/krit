package com.example

import android.content.Context

class MyProvider {
    fun setup() {
        grantUriPermissions = true
    }

    fun share(context: Context) {
        context.grantUriPermission("com.other", uri, Intent.FLAG_GRANT_READ_URI_PERMISSION)
    }
}
