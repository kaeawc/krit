package com.example

import android.content.Context
import android.content.Intent

fun open(context: Context) {
    val intent = Intent()
    context.startActivity(intent)
}
