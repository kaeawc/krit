package com.example

import android.view.View

class ViewHelper {
    fun getLeftPadding(view: View): Int {
        val field = View::class.java.getDeclaredField("mPaddingLeft")
        field.isAccessible = true
        return field.getInt(view)
    }
}
