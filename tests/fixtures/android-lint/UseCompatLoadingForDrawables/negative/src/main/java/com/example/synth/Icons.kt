package com.example.synth

import android.content.Context
import android.graphics.drawable.Drawable
import android.widget.ImageView
import androidx.appcompat.content.res.AppCompatResources
import androidx.core.content.ContextCompat
import androidx.core.content.res.ResourcesCompat

class Sprites {
    fun getDrawable(id: Int): Drawable? = null
}

class Icons(private val context: Context, private val image: ImageView) {
    private val resources = Sprites()

    fun compat(): Drawable? = AppCompatResources.getDrawable(context, android.R.drawable.ic_delete)

    fun core(): Drawable? = ContextCompat.getDrawable(context, android.R.drawable.ic_delete)

    fun res(): Drawable? = ResourcesCompat.getDrawable(context.resources, android.R.drawable.ic_delete, context.theme)

    fun shadowed(): Drawable? = resources.getDrawable(android.R.drawable.ic_delete)

    fun lookalike(): Drawable? = Sprites().getDrawable(1)

    // ImageView.getDrawable() takes no arguments.
    fun current(): Drawable? = image.drawable ?: image.getDrawable()
}
