package com.example

import android.content.Context
import android.widget.Toast

fun showMessage(context: Context) {
    Toast.makeText(context, "Hello", Toast.LENGTH_SHORT).show()
}
