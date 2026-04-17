package fixtures.positive.resourcecost

class ImageLoaderNoMemoryCache {
    fun loadImage(url: String) {
        Glide.with(context)
            .load(url)
            .skipMemoryCache(true)
            .into(imageView)
    }
}
