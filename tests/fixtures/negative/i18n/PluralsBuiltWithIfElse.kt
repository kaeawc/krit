package test

import android.content.res.Resources

class CartLabeler(private val resources: Resources) {
    fun label(count: Int): String {
        return resources.getQuantityString(R.plurals.item_count, count, count)
    }
}

// Generic identifiers: size, amount, number, num are not strong
// enough pluralization signals for this rule.
class GenericIdentifiers {
    fun tableName(size: Int): String {
        return if (size == 1) "user" else "users"
    }

    fun logTag(num: Int): String {
        return if (num == 1) "SINGLE" else "BATCH"
    }

    fun jsonKey(amount: Int): String {
        return if (amount == 1) "item" else "items"
    }

    fun describe(number: Int): String {
        return if (number == 1) "expected one" else "expected many"
    }
}

// Reversed condition: != is intentionally not matched.
class ReversedCondition {
    fun label(count: Int): String {
        return if (count != 1) "$count items" else "1 item"
    }
}

// else-if chain: outer else body is another if_expression, not a string.
class MultiQuantity {
    fun label(count: Int): String {
        return if (count == 1) "one" else if (count == 2) "a couple" else "many"
    }
}
