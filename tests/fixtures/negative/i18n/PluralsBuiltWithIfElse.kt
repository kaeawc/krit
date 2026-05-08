package test

import android.content.res.Resources

class CartLabeler(private val resources: Resources) {
    fun label(count: Int): String {
        return resources.getQuantityString(R.plurals.item_count, count, count)
    }
}
