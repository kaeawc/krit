package com.example

import android.content.Context
import android.widget.Toast

fun announceAccessibilityChange(context: Context) {
    Toast.makeText(context, "Updated", Toast.LENGTH_SHORT).show()
}
