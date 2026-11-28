package com.example

import android.content.Context
import androidx.core.content.pm.ShortcutManagerCompat

fun opened(context: Context) {
    ShortcutManagerCompat.reportShortcutUsed(context, "id")
}
