package com.example

import android.content.Intent
import android.net.Uri

fun view(uri: Uri): Intent {
    val intent = Intent()
    intent.setDataAndType(uri, "image/png")
    return intent
}
