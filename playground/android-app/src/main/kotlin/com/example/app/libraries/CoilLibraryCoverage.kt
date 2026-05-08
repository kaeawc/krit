package com.example.app.libraries

import android.content.Context
import coil3.ImageLoader
import coil3.request.CachePolicy
import coil3.request.ImageRequest

class CoilLibraryCoverage(private val context: Context) {
    val imageLoader: ImageLoader = ImageLoader.Builder(context)
        .diskCachePolicy(CachePolicy.DISABLED)
        .memoryCachePolicy(CachePolicy.DISABLED)
        .build()

    fun requestAvatar(url: String): ImageRequest {
        return ImageRequest.Builder(context)
            .data(url)
            .build()
    }
}
