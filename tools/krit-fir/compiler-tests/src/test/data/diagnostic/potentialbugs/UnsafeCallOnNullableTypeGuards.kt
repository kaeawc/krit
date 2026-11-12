// RENDER_DIAGNOSTICS_FULL_TEXT
// go-lines: 70, 75, 80, 84, 91, 96, 111, 113, 115, 118, 120
// A mutable property is never smart-cast, so an earlier precondition on it is what makes the `!!` safe.
package test.unsafecall.guards

class Holder {
    var field: String? = null
    var other: String? = null

    fun checked(): Int {
        check(field != null)
        return field!!.length
    }

    fun required(): Int {
        require(field != null && other != null)
        return field!!.length + other!!.length
    }

    fun asserted(): Int {
        assert(field != null)
        return field!!.length
    }

    fun requireNotNullGuard(): Int {
        requireNotNull(field)
        return field!!.length
    }

    fun checkNotNullGuard(): Int {
        checkNotNull(field) { "missing" }
        return field!!.length
    }

    fun elvisStatement(): Int {
        field ?: return 0
        return field!!.length
    }

    fun elvisInitializer(): Int {
        val copy = field ?: throw IllegalStateException()
        return copy.length + field!!.length
    }

    fun negatedCheck(): Int {
        check(!(field == null))
        return field!!.length
    }

    fun nullOrEmpty(): Int {
        require(!field.isNullOrEmpty())
        return field!!.length
    }

    fun explicitThis(): Int {
        check(this.field != null)
        return field!!.length
    }

    fun insideBranch(flag: Boolean): Int {
        check(field != null)
        if (flag) {
            return field!!.length
        }
        return 0
    }

    fun insideLambda(): () -> Int {
        check(field != null)
        return { field<!UnsafeCallOnNullableType!>!!<!>.length }
    }

    fun otherProperty(): Int {
        check(other != null)
        return field<!UnsafeCallOnNullableType!>!!<!>.length
    }

    fun disjunction(flag: Boolean): Int {
        check(field != null || flag)
        return field<!UnsafeCallOnNullableType!>!!<!>.length
    }

    fun afterTheUse(): Int {
        val length = field<!UnsafeCallOnNullableType!>!!<!>.length
        check(field != null)
        return length
    }

    fun checkOnAnotherReceiver(holder: Holder): Int {
        check(holder.field != null)
        return field<!UnsafeCallOnNullableType!>!!<!>.length
    }

    fun shadowed(field: String?): Int {
        check(this.field != null)
        return field<!UnsafeCallOnNullableType!>!!<!>.length
    }
}

data class Item(val name: String?, val age: Int?, val firstName: String?, val first: String?)

class Filters {
    fun sameField(list: List<Item>): List<String?> = list.filter { it.name != null }.map { it.name!! }

    fun namedParameter(list: List<Item>): List<String?> = list.filter { item -> item.name != null }.map { it.name!! }

    fun forEachAfterFilter(list: List<Item>) {
        list.filter { it.age != null }.forEach { println(it.age!!) }
    }

    fun otherField(list: List<Item>): List<String?> = list.filter { it.age != null }.map { it.name<!UnsafeCallOnNullableType!>!!<!> }

    fun otherNamedField(list: List<Item>): List<Int?> = list.filter { item -> item.name != null }.map { it.age<!UnsafeCallOnNullableType!>!!<!> }

    fun substringCollision(list: List<Item>): List<String?> = list.filter { it.firstName != null }.map { it.first<!UnsafeCallOnNullableType!>!!<!> }

    // Go reads only the callee that ends the chain, so a call after the transform is not a filtered chain.
    fun chainContinues(list: List<Item>): List<String?> = list.filter { it.name != null }.map { it.name<!UnsafeCallOnNullableType!>!!<!> }.toList()

    fun noFilter(list: List<Item>): List<String?> = list.map { it.name<!UnsafeCallOnNullableType!>!!<!> }

    fun safeCallChain(list: List<Item>?): List<String?>? = list?.filter { it.name != null }?.map { it.name!! }
}
