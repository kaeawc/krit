package fixtures.negative.resourcecost

class ImageLoaderNotInList {
    fun loadAvatar(url: String) {
        Glide.with(context)
            .load(url)
            .into(imageView)
    }
}
