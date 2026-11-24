package com.example.synth

import android.content.Context
import android.graphics.drawable.Drawable
import android.os.Bundle
import android.view.View
import androidx.appcompat.app.AppCompatActivity

class IconActivity : AppCompatActivity() {
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        val icon = getDrawable(android.R.drawable.ic_delete)
        val themed = resources.getDrawable(android.R.drawable.ic_delete, theme)
        window.decorView.setOnClickListener {
            getDrawable(android.R.drawable.ic_input_add)
        }
    }
}

fun View.badge(): Drawable? = context.getDrawable(android.R.drawable.ic_menu_add)

fun Context.placeholder(): Drawable = resources.getDrawable(android.R.drawable.ic_menu_help)
