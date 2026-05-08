package test

class CartLabeler {
    fun label(count: Int): String {
        return if (count == 1) "1 item" else "$count items"
    }
}
