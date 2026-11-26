package com.example

import android.app.Activity
import android.content.Context
import androidx.activity.ComponentActivity
import androidx.compose.runtime.Composable
import androidx.compose.ui.platform.LocalContext

class MyActivity : ComponentActivity()

@Composable
fun Casts(other: Context) {
    val a = LocalContext.current as Activity
    val b = LocalContext.current as? Activity
    val c = LocalContext.current as ComponentActivity
    val d = LocalContext.current as MyActivity
    val e = (LocalContext.current) as Activity
    val f = androidx.compose.ui.platform.LocalContext.current as Activity
    val g = other as Activity
    val ctx = LocalContext.current
    val h = ctx as Activity
    val i = LocalContext.current as Context
    val j = LocalContext.current as Any
}
