package com.example

import android.content.Context

fun wifi(context: Context) {
    context.applicationContext.getSystemService(Context.WIFI_SERVICE)
}
