package com.example

import android.content.Context
import android.content.Intent

fun open(context: Context) {
    val intent = Intent()
    intent.setAction("com.example.OPEN")
    context.startActivity(intent)
}
