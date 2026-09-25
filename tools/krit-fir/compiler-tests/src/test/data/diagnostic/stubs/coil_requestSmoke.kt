// Smoke: the ImageRequest.Builder chain.
package stubs

import android.content.Context
import coil.request.ImageRequest

fun request(context: Context, url: String): ImageRequest =
    ImageRequest.Builder(context)
        .data(url)
        .crossfade(true)
        .placeholder(android.R.drawable.ic_menu_add)
        .error(android.R.drawable.ic_menu_add)
        .build()
