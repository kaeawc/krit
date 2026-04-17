package fixtures.negative.resourcecost

class ImageLoaderNoMemoryCache {
    fun loadImage(url: String) {
        Glide.with(context)
            .load(url)
            .into(imageView)
    }
}
