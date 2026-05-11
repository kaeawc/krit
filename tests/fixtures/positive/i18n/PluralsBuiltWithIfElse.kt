package test

class CartLabeler {
    fun label(count: Int): String {
        return if (count == 1) "1 item" else "$count items"
    }

    fun quantityLabel(quantity: Int): String {
        return if (quantity == 1) "1 thing" else "$quantity things"
    }

    fun reversedOperand(count: Int): String {
        return if (1 == count) "1 widget" else "$count widgets"
    }
}
